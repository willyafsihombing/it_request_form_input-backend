package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	godotenv.Load(".env")

	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("❌ Gagal connect DB:", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("❌ DB tidak merespon:", err)
	}
	log.Println("✅ Koneksi berhasil")

	requests := []struct {
		FormNumber       string
		EffectiveDate    string
		ReqCompany       string
		ReqName          string
		RecCompany       string
		RecDept          string
		RecLocation      string
		RecPersonnel     string
		RecEmpId         string
		RecTitle         string
		RecStatus        string
		SnAdd            bool
		SnLAN            bool
		SnVPN            bool
		SnEmail          bool
		HwAdd            bool
		HwPC             bool
		HwNotebook       bool
		HwMSOffice       bool
		HwOther          string
		ErpAdd           bool
		ErpPronto        bool
		ErpSmartMining   bool
		ErpPositionId    string
		ErpSignOnId      string
		ErpGlobalProfile string
		AdditionalDesc   string
		CostCode         string
		Justification    string
		SigRequester     string
		SigITAdmin       string
		SigMSDIMgr       string
		Status           string
		Priority         string
		ItNotes          string
		AssignedTo       string
		CreatedAt        time.Time
	}{
		{
			FormNumber: "175-ITR-Unggul062026", EffectiveDate: "6/1/2026",
			ReqCompany: "PT. UNGGUL DINAMIKA UTAMA", ReqName: "Willy Sihombing",
			RecCompany: "PT. UNGGUL DINAMIKA UTAMA", RecDept: "IT",
			RecLocation: "SITE INDEXIM - KALIORANG", RecPersonnel: "Hendrik Adelmei",
			RecEmpId: "502535356", RecTitle: "IT Staff", RecStatus: "KARYAWAN",
			SnAdd: true, SnLAN: true, SnVPN: true, SnEmail: true,
			HwAdd: true, HwPC: true, HwNotebook: false, HwMSOffice: true, HwOther: "",
			ErpAdd: false, ErpPronto: false, ErpSmartMining: false,
			ErpPositionId: "", ErpSignOnId: "", ErpGlobalProfile: "",
			AdditionalDesc: "Request laptop baru beserta instalasi Windows dan MS Office",
			CostCode:       "IT-2026-001", Justification: "Kebutuhan kerja lapangan",
			SigRequester: "Willy Sihombing", SigITAdmin: "Hendrik Adelmei", SigMSDIMgr: "",
			Status: "approved", Priority: "high",
			ItNotes: "Sudah disetujui", AssignedTo: "Hendrik Adelmei",
			CreatedAt: time.Now().AddDate(0, 0, -5),
		},
		{
			FormNumber: "176-ITR-Unggul062026", EffectiveDate: "6/2/2026",
			ReqCompany: "PT. UNGGUL DINAMIKA UTAMA", ReqName: "Surya Saputra",
			RecCompany: "PT. UNGGUL DINAMIKA UTAMA", RecDept: "GA",
			RecLocation: "SITE INDEXIM - KALIORANG", RecPersonnel: "Ferri Mulianto",
			RecEmpId: "51122646", RecTitle: "ACT. SUPT GA", RecStatus: "KARYAWAN",
			SnAdd: true, SnLAN: false, SnVPN: false, SnEmail: false,
			HwAdd: true, HwPC: true, HwNotebook: false, HwMSOffice: false, HwOther: "LAPTOP",
			ErpAdd: false, ErpPronto: false, ErpSmartMining: false,
			ErpPositionId: "", ErpSignOnId: "", ErpGlobalProfile: "",
			AdditionalDesc: "1x Laptop Legion beserta Windows dan Office serta Mouse",
			CostCode:       "", Justification: "Untuk kerja",
			SigRequester: "Surya Saputra", SigITAdmin: "", SigMSDIMgr: "",
			Status: "pending", Priority: "normal",
			ItNotes: "", AssignedTo: "",
			CreatedAt: time.Now().AddDate(0, 0, -4),
		},
		{
			FormNumber: "177-ITR-Unggul062026", EffectiveDate: "6/3/2026",
			ReqCompany: "PT. UNGGUL DINAMIKA UTAMA", ReqName: "Albertus P",
			RecCompany: "PT. UNGGUL DINAMIKA UTAMA", RecDept: "MSDI",
			RecLocation: "SITE INDEXIM - KALIORANG", RecPersonnel: "Budi Santoso",
			RecEmpId: "503421789", RecTitle: "SUPT IT", RecStatus: "KARYAWAN",
			SnAdd: false, SnLAN: false, SnVPN: false, SnEmail: true,
			HwAdd: false, HwPC: false, HwNotebook: false, HwMSOffice: false, HwOther: "",
			ErpAdd: true, ErpPronto: true, ErpSmartMining: false,
			ErpPositionId: "POS-001", ErpSignOnId: "budi.santoso", ErpGlobalProfile: "PROFILE-STD",
			AdditionalDesc: "Setup akses PRONTO untuk staff MSDI baru",
			CostCode:       "MSDI-2026-001", Justification: "Karyawan baru mulai 3 Juni 2026",
			SigRequester: "Albertus P", SigITAdmin: "Hendrik Adelmei", SigMSDIMgr: "Ade Wahyudin",
			Status: "in_review", Priority: "urgent",
			ItNotes: "Sedang diproses tim IT", AssignedTo: "Hendrik Adelmei",
			CreatedAt: time.Now().AddDate(0, 0, -3),
		},
		{
			FormNumber: "178-ITR-Unggul062026", EffectiveDate: "6/4/2026",
			ReqCompany: "PT. UNGGUL DINAMIKA UTAMA", ReqName: "Jason Wijaya",
			RecCompany: "PT. UNGGUL DINAMIKA UTAMA", RecDept: "Finance",
			RecLocation: "HEAD OFFICE - JAKARTA", RecPersonnel: "Rina Kusuma",
			RecEmpId: "504789123", RecTitle: "Finance Manager", RecStatus: "KARYAWAN",
			SnAdd: true, SnLAN: false, SnVPN: true, SnEmail: true,
			HwAdd: false, HwPC: false, HwNotebook: false, HwMSOffice: true, HwOther: "",
			ErpAdd: true, ErpPronto: false, ErpSmartMining: true,
			ErpPositionId: "POS-002", ErpSignOnId: "rina.kusuma", ErpGlobalProfile: "PROFILE-FIN",
			AdditionalDesc: "Setup akses SMART MINING dan VPN untuk Finance Manager baru",
			CostCode:       "FIN-2026-001", Justification: "Karyawan baru mulai 4 Juni 2026",
			SigRequester: "Jason Wijaya", SigITAdmin: "", SigMSDIMgr: "",
			Status: "pending", Priority: "high",
			ItNotes: "", AssignedTo: "",
			CreatedAt: time.Now().AddDate(0, 0, -2),
		},
		{
			FormNumber: "179-ITR-Unggul062026", EffectiveDate: "6/5/2026",
			ReqCompany: "PT. UNGGUL DINAMIKA UTAMA", ReqName: "Ade Wahyudin",
			RecCompany: "PT. UNGGUL DINAMIKA UTAMA", RecDept: "Operation",
			RecLocation: "SITE INDEXIM - KALIORANG", RecPersonnel: "Doni Pratama",
			RecEmpId: "505234567", RecTitle: "Operation Supervisor", RecStatus: "KARYAWAN",
			SnAdd: false, SnLAN: false, SnVPN: false, SnEmail: false,
			HwAdd: false, HwPC: false, HwNotebook: false, HwMSOffice: true, HwOther: "",
			ErpAdd: false, ErpPronto: false, ErpSmartMining: false,
			ErpPositionId: "", ErpSignOnId: "", ErpGlobalProfile: "",
			AdditionalDesc: "Update lisensi MS Office Suite dari versi lama ke Office 365",
			CostCode:       "OPS-2026-002", Justification: "Upgrade lisensi tahunan",
			SigRequester: "Ade Wahyudin", SigITAdmin: "Hendrik Adelmei", SigMSDIMgr: "Ade Wahyudin",
			Status: "approved", Priority: "normal",
			ItNotes: "Upgrade selesai dilakukan", AssignedTo: "Hendrik Adelmei",
			CreatedAt: time.Now().AddDate(0, 0, -1),
		},
	}

	for _, req := range requests {
		var count int
		db.QueryRow(
			"SELECT * COUNT(*) FROM requests WHERE form_number = $1",
			req.FormNumber,
		).Scan(&count)

		if count > 0 {
			log.Printf("⚠️  Request %s already, skip", req.FormNumber)
			continue
		}
		id := uuid.New().String()
		now := time.Now()

		_, err := db.Exec(`
			INSERT INTO requests (
				id, form_number, effective_date,
				req_company, req_name, rec_company, rec_dept,
				rec_location, rec_personnel, rec_emp_id, rec_title, rec_status,
				sn_add, sn_lan, sn_vpn, sn_email,
				sn_change, sn_terminate, sn_file_sharing, sn_intranet, sn_other,
				hw_add, hw_pc, hw_notebook, hw_ms_office, hw_other,
				hw_change, hw_terminate, hw_printer, hw_adobe, hw_zoom,
				erp_add, erp_pronto, erp_smart_mining,
				erp_position_id, erp_sign_on_id, erp_global_profile,
				erp_change, erp_terminate, erp_district, erp_ref,
				additional_desc, cost_code, justification, tech_comment,
				tech_ad, tech_aw, tech_cr, tech_em, tech_ip, tech_ep,
				sig_requester, sig_spt_dept, sig_dept_mgr, sig_sr_mgr,
				sig_hoo, sig_it_admin, sig_msdi_mgr,
				status, priority, it_notes, assigned_to,
				created_at, updated_at
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,
				$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,
				$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,
				$31,$32,$33,$34,$35,$36,$37,$38,$39,$40,
				$41,$42,$43,$44,$45,$46,$47,$48,$49,$50,
				$51,$52,$53,$54,$55,$56,$57,$58,$59,$60,
				$61,$62,$63,$64
			)
		`,
			id, req.FormNumber, req.EffectiveDate,
			req.ReqCompany, req.ReqName, req.RecCompany, req.RecDept,
			req.RecLocation, req.RecPersonnel, req.RecEmpId, req.RecTitle, req.RecStatus,
			req.SnAdd, req.SnLAN, req.SnVPN, req.SnEmail,
			false, false, false, false, "",
			req.HwAdd, req.HwPC, req.HwNotebook, req.HwMSOffice, req.HwOther,
			false, false, false, false, false,
			req.ErpAdd, req.ErpPronto, req.ErpSmartMining,
			req.ErpPositionId, req.ErpSignOnId, req.ErpGlobalProfile,
			false, false, "", "",
			req.AdditionalDesc, req.CostCode, req.Justification, "",
			false, false, false, false, false, false,
			req.SigRequester, "", "", "",
			"", req.SigITAdmin, req.SigMSDIMgr,
			req.Status, req.Priority, req.ItNotes, req.AssignedTo,
			req.CreatedAt, now,
		)

		if err != nil {
			log.Printf("❌ Gagal insert %s: %v", req.FormNumber, err)
			continue
		}

		log.Printf("✅ Request created: %s — %s", id, req.FormNumber)
	}

	fmt.Println("🎉 Seed request selesai")
}
