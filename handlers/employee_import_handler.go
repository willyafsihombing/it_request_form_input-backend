package handlers

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"unicode"

	"it-request-backend/models"
	"it-request-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

const (
	maxImportFileBytes = 10 << 20
	maxErrorsReturned  = 100
)

type importRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

type importReport struct {
	DryRun     bool             `json:"dryRun"`
	Total      int              `json:"total"`
	Valid      int              `json:"valid"`
	ToCreate   int              `json:"toCreate"`
	ToUpdate   int              `json:"toUpdate"`
	Created    int              `json:"created"`
	Updated    int              `json:"updated"`
	ErrorCount int              `json:"errorCount"`
	Errors     []importRowError `json:"errors"`
	Error      string           `json:"error,omitempty"`
}

type parsedEmployeeSheet struct {
	Rows       []models.EmployeeRequest
	Total      int
	Errors     []importRowError
	ErrorCount int
}

var importHeaderAliases = map[string]string{
	"nik": "nik", "nip": "nik",
	"nama": "nama", "namakaryawan": "nama", "namalengkap": "nama", "name": "nama", "fullname": "nama",
	"jabatan": "jabatan", "title": "jabatan", "position": "jabatan",
	"departemen": "departemen", "departmen": "departemen", "dept": "departemen", "department": "departemen",
	"level": "level", "grade": "level",
	"staffnonstaff": "stafftype", "stafftype": "stafftype", "tipe": "stafftype", "staff": "stafftype", "kategori": "stafftype",
}

func normHeader(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func parseEmployeeExcel(src io.Reader) (*parsedEmployeeSheet, error) {
	f, err := excelize.OpenReader(src)
	if err != nil {
		return nil, errors.New("File tidak bisa dibaca. Gunakan file Excel berformat .xlsx")
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("File Excel tidak memiliki sheet")
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, errors.New("Gagal membaca isi sheet pertama")
	}
	if len(rows) < 2 {
		return nil, errors.New("File tidak berisi data (hanya judul kolom, atau kosong)")
	}

	col := map[string]int{}
	for i, h := range rows[0] {
		if field, ok := importHeaderAliases[normHeader(h)]; ok {
			if _, dup := col[field]; !dup {
				col[field] = i
			}
		}
	}
	var missing []string
	if _, ok := col["nik"]; !ok {
		missing = append(missing, "NIK")
	}
	if _, ok := col["nama"]; !ok {
		missing = append(missing, "Nama")
	}
	if _, ok := col["stafftype"]; !ok {
		missing = append(missing, "Staff / Non Staff")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf(
			"Kolom wajib tidak ditemukan di baris pertama: %s. Gunakan template yang disediakan",
			strings.Join(missing, ", "))
	}

	cell := func(line []string, field string) string {
		i, ok := col[field]
		if !ok || i >= len(line) {
			return ""
		}
		return strings.TrimSpace(line[i])
	}

	res := &parsedEmployeeSheet{}
	seen := map[string]int{}
	addErr := func(row int, msg string) {
		res.ErrorCount++
		if len(res.Errors) < maxErrorsReturned {
			res.Errors = append(res.Errors, importRowError{Row: row, Message: msg})
		}
	}

	for i := 1; i < len(rows); i++ {
		rowNum := i + 1
		line := rows[i]

		nik := cell(line, "nik")
		nama := cell(line, "nama")
		jabatan := cell(line, "jabatan")
		dept := cell(line, "departemen")
		level := cell(line, "level")
		tipeRaw := cell(line, "stafftype")

		if nik == "" && nama == "" && jabatan == "" && dept == "" && level == "" && tipeRaw == "" {
			continue
		}

		res.Total++
		if res.Total > maxImportRows {
			return nil, fmt.Errorf("Maksimal %d baris data per file", maxImportRows)
		}

		staffType := normalizeStaffType(tipeRaw)
		bad := false

		if nik == "" {
			addErr(rowNum, "NIK kosong")
			bad = true
		}
		if nama == "" {
			addErr(rowNum, "Nama kosong")
			bad = true
		}
		if staffType == "" {
			addErr(rowNum, fmt.Sprintf(`Kolom Staff / Non Staff harus "Staff" atau "Non Staff" (terbaca: %q)`, tipeRaw))
			bad = true
		}
		if nik != "" {
			if first, dup := seen[nik]; dup {
				addErr(rowNum, fmt.Sprintf("NIK %s duplikat dengan baris %d", nik, first))
				bad = true
			} else {
				seen[nik] = rowNum
			}
		}
		if bad {
			continue
		}

		res.Rows = append(res.Rows, models.EmployeeRequest{
			NIK:        nik,
			Nama:       nama,
			Jabatan:    jabatan,
			Departemen: dept,
			Level:      level,
			StaffType:  staffType,
		})
	}

	return res, nil
}

func (h *EmployeeHandler) ImportExcel(c *gin.Context) {
	if !h.requireManage(c) {
		return
	}
	dryRun := c.Query("dryRun") == "true"

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImportFileBytes)
	fh, err := c.FormFile("file")
	if err != nil {
		utils.BadRequest(c, "File tidak ditemukan atau terlalu besar (maksimal 10 MB)")
		return
	}
	if strings.ToLower(filepath.Ext(fh.Filename)) != ".xlsx" {
		utils.BadRequest(c, "Format file harus .xlsx (Excel)")
		return
	}

	file, err := fh.Open()
	if err != nil {
		utils.InternalError(c, "Gagal membuka file")
		return
	}
	defer file.Close()

	sheet, err := parseEmployeeExcel(file)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if sheet.Total == 0 {
		utils.BadRequest(c, "Tidak ada baris data di file")
		return
	}

	niks := make([]string, 0, len(sheet.Rows))
	for _, row := range sheet.Rows {
		niks = append(niks, row.NIK)
	}
	existing, err := h.Repo.ExistingNIKs(niks)
	if err != nil {
		log.Println("import excel: cek NIK error:", err)
		utils.InternalError(c, "Gagal memeriksa data yang sudah ada")
		return
	}
	toUpdate := 0
	for _, n := range niks {
		if existing[n] {
			toUpdate++
		}
	}

	report := importReport{
		DryRun:     dryRun,
		Total:      sheet.Total,
		Valid:      len(sheet.Rows),
		ToCreate:   len(sheet.Rows) - toUpdate,
		ToUpdate:   toUpdate,
		ErrorCount: sheet.ErrorCount,
		Errors:     sheet.Errors,
	}
	if report.Errors == nil {
		report.Errors = []importRowError{}
	}

	if dryRun {
		utils.OK(c, report)
		return
	}

	if sheet.ErrorCount > 0 {
		report.Error = fmt.Sprintf("Impor dibatalkan: %d baris bermasalah. Perbaiki file lalu coba lagi", sheet.ErrorCount)
		c.JSON(http.StatusBadRequest, report)
		return
	}

	created, updated, err := h.Repo.ImportMany(sheet.Rows)
	if err != nil {
		log.Println("import excel: simpan error:", err)
		utils.InternalError(c, "Gagal menyimpan data karyawan")
		return
	}
	report.Created, report.Updated = created, updated
	utils.OK(c, report)
}

func (h *EmployeeHandler) ImportTemplate(c *gin.Context) {
	if !h.requireManage(c) {
		return
	}

	f := excelize.NewFile()
	defer f.Close()

	const sheet = "Karyawan"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		log.Println("template karyawan error:", err)
		utils.InternalError(c, "Gagal membuat template")
		return
	}

	headers := []string{"NIK", "Nama", "Jabatan", "Departemen", "Level", "Staff / Non Staff"}
	examples := [][]string{
		{"100001", "Contoh Karyawan Satu", "IT Staff", "IT", "3", "Staff"},
		{"100002", "Contoh Karyawan Dua", "Operator", "HR", "1", "Non Staff"},
	}

	textStyle, _ := f.NewStyle(&excelize.Style{NumFmt: 49})
	_ = f.SetColStyle(sheet, "A", textStyle)

	for i, hd := range headers {
		cellName, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cellName, hd)
	}
	for r, row := range examples {
		for col, v := range row {
			cellName, _ := excelize.CoordinatesToCellName(col+1, r+2)
			_ = f.SetCellValue(sheet, cellName, v)
		}
	}

	headStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"0284C7"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	_ = f.SetCellStyle(sheet, "A1", "F1", headStyle)

	widths := map[string]float64{"A": 16, "B": 32, "C": 26, "D": 20, "E": 10, "F": 20}
	for colName, w := range widths {
		_ = f.SetColWidth(sheet, colName, colName, w)
	}

	const guide = "Petunjuk"
	if _, err := f.NewSheet(guide); err == nil {
		lines := []string{
			"PETUNJUK PENGISIAN",
			"1. Isi data di sheet \"Karyawan\" mulai baris 2. Hapus dua baris contoh sebelum mengunggah.",
			"2. Jangan ubah judul kolom di baris 1.",
			"3. Kolom wajib: NIK, Nama, dan Staff / Non Staff. Kolom lain boleh kosong.",
			"4. Kolom Staff / Non Staff diisi \"Staff\" atau \"Non Staff\".",
			"5. NIK yang sudah terdaftar akan DIPERBARUI (tidak digandakan).",
			"6. Satu NIK hanya boleh muncul sekali di dalam file.",
			"7. Maksimal 5.000 baris data per file, format .xlsx.",
			"8. Biarkan kolom NIK berformat teks agar angka nol di depan tidak hilang.",
		}
		for i, line := range lines {
			cellName, _ := excelize.CoordinatesToCellName(1, i+1)
			_ = f.SetCellValue(guide, cellName, line)
		}
		_ = f.SetColWidth(guide, "A", "A", 95)
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", `attachment; filename="template-karyawan.xlsx"`)
	c.Status(http.StatusOK)
	if err := f.Write(c.Writer); err != nil {
		log.Println("template karyawan error:", err)
	}
}
