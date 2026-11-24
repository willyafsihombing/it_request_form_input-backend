package itrequest

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type RequestRepository struct {
	DB *sql.DB
}

func NewRequestRepository(db *sql.DB) *RequestRepository {
	return &RequestRepository{DB: db}
}

type ViewScope struct {
	All        bool   // admin / Manager IT
	UserID     string // pembuat request
	EmployeeID string // approver yang ditunjuk di rantai
	Dept       string // departemen user (employees.departemen)
}

func (s ViewScope) Clause(argIdx int) (string, []interface{}) {
	if s.All {
		return "", nil
	}
	c := fmt.Sprintf(`(
		requests.created_by_user_id::text = $%[1]d::text
		OR ($%[2]d::text <> '' AND EXISTS (
			SELECT 1 FROM users cu
			JOIN employees ce ON ce.id = cu.employee_id
			WHERE cu.id::text = requests.created_by_user_id::text
			  AND ce.departemen = $%[2]d::text))
		OR ($%[3]d::text <> '' AND EXISTS (
			SELECT 1 FROM approval_steps s
			JOIN approval_step_approvers a ON a.step_id = s.id
			WHERE s.request_id = requests.id
			  AND a.employee_id::text = $%[3]d::text))
	)`, argIdx, argIdx+1, argIdx+2)
	return c, []interface{}{s.UserID, s.Dept, s.EmployeeID}
}

// CanView dipakai GetByID di handler.
func (r *RequestRepository) CanView(id string, s ViewScope) (bool, error) {
	args := []interface{}{id}
	q := "SELECT COUNT(*) FROM requests WHERE id = $1 AND deleted_at IS NULL"
	if cl, a := s.Clause(2); cl != "" {
		q += " AND " + cl
		args = append(args, a...)
	}
	var n int
	err := r.DB.QueryRow(q, args...).Scan(&n)
	return n > 0, err
}

// GET all dengan filter, search, pagination
func (r *RequestRepository) FindAll(status, priority, search string, page, limit int, scope ViewScope) ([]RequestListItem, int, error) {
	conditions := []string{"deleted_at IS NULL"}
	args := []interface{}{}
	argIdx := 1

	if status != "" && status != "all" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}

	if priority != "" && priority != "all" {
		conditions = append(conditions, fmt.Sprintf("priority = $%d", argIdx))
		args = append(args, priority)
		argIdx++
	}

	if search != "" {
		conditions = append(conditions, fmt.Sprintf(`(
			form_number   ILIKE $%[1]d OR
			request_title ILIKE $%[1]d OR
			req_name      ILIKE $%[1]d OR
			rec_personnel ILIKE $%[1]d OR
			rec_dept      ILIKE $%[1]d OR
			rec_location  ILIKE $%[1]d
		)`, argIdx))
		args = append(args, "%"+search+"%")
		argIdx++
	}

	if cl, a := scope.Clause(argIdx); cl != "" {
		conditions = append(conditions, cl)
		args = append(args, a...)
		argIdx += len(a)
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	// Count total
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM requests %s", where)
	if err := r.DB.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Fetch data
	offset := (page - 1) * limit
	args = append(args, limit, offset)
	query := fmt.Sprintf(`
        SELECT
            id, form_number, effective_date, request_title,
            req_name, rec_dept, rec_location,
            rec_personnel, rec_emp_id,
            hw_add, hw_change, hw_terminate,
            sn_add, sn_change, sn_terminate,
            erp_add, erp_change, erp_terminate,
            hw_pc, hw_printer, hw_notebook, hw_other,
            additional_desc, status, priority,
            assigned_to, created_at, updated_at,
            COALESCE(approval_status, ''), current_step,
            COALESCE(created_by_user_id::text, '')
        FROM requests
        %s
        ORDER BY created_at DESC
        LIMIT $%d OFFSET $%d
    `, where, argIdx, argIdx+1)

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []RequestListItem
	for rows.Next() {
		var item RequestListItem
		err := rows.Scan(
			&item.ID, &item.FormNumber, &item.EffectiveDate, &item.RequestTitle,
			&item.ReqName, &item.RecDept, &item.RecLocation,
			&item.RecPersonnel, &item.RecEmpId,
			&item.HwAdd, &item.HwChange, &item.HwTerminate,
			&item.SnAdd, &item.SnChange, &item.SnTerminate,
			&item.ErpAdd, &item.ErpChange, &item.ErpTerminate,
			&item.HwPC, &item.HwPrinter, &item.HwNotebook, &item.HwOther,
			&item.AdditionalDesc, &item.Status, &item.Priority,
			&item.AssignedTo, &item.CreatedAt, &item.UpdatedAt,
			&item.ApprovalStatus, &item.CurrentStep, &item.CreatedByUserID,
		)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *RequestRepository) FindByID(id string) (*Request, error) {
	var req Request
	var deletedAt sql.NullTime
	var createdBy sql.NullString

	query := `
		SELECT
			id, form_number, effective_date, request_title,
			req_company, req_name,
			rec_company, rec_dept, rec_location, rec_personnel,
			rec_emp_id, rec_title, rec_status,
			sn_add, sn_change, sn_terminate,
			sn_lan, sn_vpn, sn_email, sn_file_sharing, sn_intranet, sn_other,
			hw_add, hw_change, hw_terminate,
			hw_pc, hw_printer, hw_notebook, hw_ms_office, hw_adobe, hw_zoom, hw_other,
			erp_add, erp_change, erp_terminate,
			erp_pronto, erp_smart_mining,
			erp_position_id, erp_district, erp_ref, erp_sign_on_id, erp_global_profile,
			additional_desc, cost_code, justification, tech_comment,
			tech_ad, tech_aw, tech_cr, tech_em, tech_ip, tech_ep,
			sig_requester, sig_spt_dept, sig_dept_mgr, sig_sr_mgr,
			sig_hoo, sig_it_admin, sig_msdi_mgr,
			status, priority, it_notes, assigned_to,
			COALESCE(approval_status, ''), current_step, approval_round,
			deleted_at, created_at, updated_at, created_by_user_id
		FROM requests
		WHERE id = $1 AND deleted_at IS NULL
	`

	err := r.DB.QueryRow(query, id).Scan(
		&req.ID, &req.FormNumber, &req.EffectiveDate, &req.RequestTitle,
		&req.ReqCompany, &req.ReqName,
		&req.RecCompany, &req.RecDept, &req.RecLocation, &req.RecPersonnel,
		&req.RecEmpId, &req.RecTitle, &req.RecStatus,
		&req.SnAdd, &req.SnChange, &req.SnTerminate,
		&req.SnLAN, &req.SnVPN, &req.SnEmail, &req.SnFileSharing, &req.SnIntranet, &req.SnOther,
		&req.HwAdd, &req.HwChange, &req.HwTerminate,
		&req.HwPC, &req.HwPrinter, &req.HwNotebook, &req.HwMSOffice, &req.HwAdobe, &req.HwZoom, &req.HwOther,
		&req.ErpAdd, &req.ErpChange, &req.ErpTerminate,
		&req.ErpPronto, &req.ErpSmartMining,
		&req.ErpPositionId, &req.ErpDistrict, &req.ErpRef, &req.ErpSignOnId, &req.ErpGlobalProfile,
		&req.AdditionalDesc, &req.CostCode, &req.Justification, &req.TechComment,
		&req.TechAD, &req.TechAW, &req.TechCR, &req.TechEM, &req.TechIP, &req.TechEP,
		&req.SigRequester, &req.SigSptDept, &req.SigDeptMgr, &req.SigSrMgr,
		&req.SigHOO, &req.SigITAdmin, &req.SigMSDIMgr,
		&req.Status, &req.Priority, &req.ItNotes, &req.AssignedTo,
		&req.ApprovalStatus, &req.CurrentStep, &req.ApprovalRound,
		&deletedAt, &req.CreatedAt, &req.UpdatedAt, &createdBy,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if deletedAt.Valid {
		req.DeletedAt = &deletedAt.Time
	}

	if createdBy.Valid {
		req.CreatedByUserID = &createdBy.String
	}

	return &req, nil
}

// ExistsByID — cek ada dan belum dihapus
func (r *RequestRepository) ExistsByID(id string) (bool, error) {
	var count int
	err := r.DB.QueryRow(
		"SELECT COUNT(*) FROM requests WHERE id = $1 AND deleted_at IS NULL",
		id,
	).Scan(&count)
	return count > 0, err
}

type dbExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func (r *RequestRepository) Create(req *Request) error {
	return r.create(r.DB, req)
}

func (r *RequestRepository) CreateInTx(req *Request, after func(tx *sql.Tx) error) error {
	tx, err := r.DB.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	if err := r.create(tx, req); err != nil {
		return err
	}

	if after != nil {
		if err := after(tx); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// POST — create request baru
func (r *RequestRepository) create(db dbExecer, req *Request) error {
	query := `
        INSERT INTO requests (
            id, form_number,request_title, effective_date,
            req_company, req_name, rec_company, rec_dept,
            rec_location, rec_personnel, rec_emp_id, rec_title, rec_status,
            sn_add, sn_change, sn_terminate, sn_lan, sn_vpn, sn_email,
            sn_file_sharing, sn_intranet, sn_other,
            hw_add, hw_change, hw_terminate, hw_pc, hw_printer,
            hw_notebook, hw_ms_office, hw_adobe, hw_zoom, hw_other,
            erp_add, erp_change, erp_terminate, erp_pronto, erp_smart_mining,
            erp_position_id, erp_district, erp_ref, erp_sign_on_id, erp_global_profile,
            additional_desc, cost_code, justification, tech_comment,
            tech_ad, tech_aw, tech_cr, tech_em, tech_ip, tech_ep,
            sig_requester, sig_spt_dept, sig_dept_mgr, sig_sr_mgr,
            sig_hoo, sig_it_admin, sig_msdi_mgr,
            status, priority, created_at, updated_at, created_by_user_id
        ) VALUES (
            $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,
            $11,$12,$13,$14,$15,$16,$17,$18,$19,$20,
            $21,$22,$23,$24,$25,$26,$27,$28,$29,$30,
            $31,$32,$33,$34,$35,$36,$37,$38,$39,$40,
            $41,$42,$43,$44,$45,$46,$47,$48,$49,$50,
            $51,$52,$53,$54,$55,$56,$57,$58,$59,$60,
            $61,$62,$63,$64
        )
    `
	now := time.Now()
	_, err := db.Exec(query,
		req.ID, req.FormNumber, req.RequestTitle, req.EffectiveDate,
		req.ReqCompany, req.ReqName, req.RecCompany, req.RecDept,
		req.RecLocation, req.RecPersonnel, req.RecEmpId, req.RecTitle, req.RecStatus,
		req.SnAdd, req.SnChange, req.SnTerminate, req.SnLAN, req.SnVPN, req.SnEmail,
		req.SnFileSharing, req.SnIntranet, req.SnOther,
		req.HwAdd, req.HwChange, req.HwTerminate, req.HwPC, req.HwPrinter,
		req.HwNotebook, req.HwMSOffice, req.HwAdobe, req.HwZoom, req.HwOther,
		req.ErpAdd, req.ErpChange, req.ErpTerminate, req.ErpPronto, req.ErpSmartMining,
		req.ErpPositionId, req.ErpDistrict, req.ErpRef, req.ErpSignOnId, req.ErpGlobalProfile,
		req.AdditionalDesc, req.CostCode, req.Justification, req.TechComment,
		req.TechAD, req.TechAW, req.TechCR, req.TechEM, req.TechIP, req.TechEP,
		req.SigRequester, req.SigSptDept, req.SigDeptMgr, req.SigSrMgr,
		req.SigHOO, req.SigITAdmin, req.SigMSDIMgr,
		req.Status, req.Priority, now, now, req.CreatedByUserID,
	)
	return err
}

func (r *RequestRepository) Resubmit(id string, q *Request) (bool, error) {
	return r.resubmit(r.DB, id, q)
}

func (r *RequestRepository) ResubmitInTx(id string, q *Request, after func(tx *sql.Tx) error) (bool, error) {
	tx, err := r.DB.Begin()
	if err != nil {
		return false, err
	}

	defer tx.Rollback()

	ok, err := r.resubmit(tx, id, q)
	if err != nil || !ok {
		return false, err
	}

	if after != nil {
		if err := after(tx); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

func (r *RequestRepository) resubmit(db dbExecer, id string, q *Request) (bool, error) {
	cols := []struct {
		col string
		val any
	}{
		{"request_title", q.RequestTitle},
		{"req_company", q.ReqCompany},
		{"rec_company", q.RecCompany},
		{"rec_dept", q.RecDept},
		{"rec_location", q.RecLocation},
		{"rec_personnel", q.RecPersonnel},
		{"rec_emp_id", q.RecEmpId},
		{"rec_title", q.RecTitle},
		{"rec_status", q.RecStatus},
		{"sn_add", q.SnAdd}, {"sn_change", q.SnChange}, {"sn_terminate", q.SnTerminate},
		{"sn_lan", q.SnLAN}, {"sn_vpn", q.SnVPN}, {"sn_email", q.SnEmail},
		{"sn_file_sharing", q.SnFileSharing}, {"sn_intranet", q.SnIntranet}, {"sn_other", q.SnOther},
		{"hw_add", q.HwAdd}, {"hw_change", q.HwChange}, {"hw_terminate", q.HwTerminate},
		{"hw_pc", q.HwPC}, {"hw_printer", q.HwPrinter}, {"hw_notebook", q.HwNotebook},
		{"hw_ms_office", q.HwMSOffice}, {"hw_adobe", q.HwAdobe}, {"hw_zoom", q.HwZoom}, {"hw_other", q.HwOther},
		{"erp_add", q.ErpAdd}, {"erp_change", q.ErpChange}, {"erp_terminate", q.ErpTerminate},
		{"erp_pronto", q.ErpPronto}, {"erp_smart_mining", q.ErpSmartMining},
		{"erp_position_id", q.ErpPositionId}, {"erp_district", q.ErpDistrict}, {"erp_ref", q.ErpRef},
		{"erp_sign_on_id", q.ErpSignOnId}, {"erp_global_profile", q.ErpGlobalProfile},
		{"additional_desc", q.AdditionalDesc},
		{"cost_code", q.CostCode},
		{"justification", q.Justification},
		{"sig_spt_dept", q.SigSptDept}, {"sig_dept_mgr", q.SigDeptMgr},
		{"sig_sr_mgr", q.SigSrMgr}, {"sig_hoo", q.SigHOO},
		{"priority", q.Priority},
	}

	set := make([]string, 0, len(cols)+2)
	args := make([]any, 0, len(cols)+1)
	args = append(args, id) // $1
	for i, c := range cols {
		set = append(set, fmt.Sprintf("%s = $%d", c.col, i+2))
		args = append(args, c.val)
	}

	query := `UPDATE requests SET ` + strings.Join(set, ", ") +
		`, status = 'pending', updated_at = now()
		 WHERE id = $1 AND status = 'rejected' AND deleted_at IS NULL`

	res, err := db.Exec(query, args...)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *RequestRepository) RequestOrg(userID string) (employeeID, departemen string, err error) {
	if userID == "" {
		return "", "", nil
	}
	err = r.DB.QueryRow(`
		SELECT COALESCE (u.employee_id::text, ''), COALESCE(e.departemen, '')
		FROM users u
		LEFT JOIN employees e ON e.id = u.employee_id
		WHERE u.id = $1`, userID).Scan(&employeeID, &departemen)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	return employeeID, departemen, err
}

// Cek duplikat form number
func (r *RequestRepository) ExistsByFormNumber(formNumber string) (bool, error) {
	var count int
	err := r.DB.QueryRow(
		"SELECT COUNT(*) FROM requests WHERE form_number = $1 AND deleted_at IS NULL",
		formNumber,
	).Scan(&count)
	return count > 0, err
}

func (r *RequestRepository) Update(id string, fields map[string]interface{}) error {
	fieldMap := map[string]string{
		"requestTitle":   "request_title",
		"status":         "status",
		"priority":       "priority",
		"itNotes":        "it_notes",
		"assignedTo":     "assigned_to",
		"techComment":    "tech_comment",
		"techAD":         "tech_ad",
		"techAW":         "tech_aw",
		"techCR":         "tech_cr",
		"techEM":         "tech_em",
		"techIP":         "tech_ip",
		"techEP":         "tech_ep",
		"sigITAdmin":     "sig_it_admin",
		"sigMSDIMgr":     "sig_msdi_mgr",
		"sigSrMgr":       "sig_sr_mgr",
		"sigHOO":         "sig_hoo",
		"costCode":       "cost_code",
		"justification":  "justification",
		"additionalDesc": "additional_desc",
	}

	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	for camel, snake := range fieldMap {
		if val, ok := fields[camel]; ok {
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", snake, argIdx))
			args = append(args, val)
			argIdx++
		}
	}

	if len(setClauses) == 0 {
		return nil
	}

	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argIdx))
	args = append(args, time.Now())
	argIdx++

	args = append(args, id)
	query := fmt.Sprintf(
		"UPDATE requests SET %s WHERE id = $%d AND deleted_at IS NULL",
		strings.Join(setClauses, ", "),
		argIdx,
	)

	_, err := r.DB.Exec(query, args...)
	return err
}

// SoftDelete — set deleted_at
func (r *RequestRepository) SoftDelete(id string) error {
	_, err := r.DB.Exec(
		"UPDATE requests SET deleted_at = $1, updated_at = $1 WHERE id = $2 AND deleted_at IS NULL",
		time.Now(), id,
	)
	return err
}
