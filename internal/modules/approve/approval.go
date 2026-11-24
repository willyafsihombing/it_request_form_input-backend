package approve

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"strings"
	"time"
)

type StepCode string

const (
	StepITCheck     StepCode = "it_check"
	StepITManager   StepCode = "it_manager"
	StepSuptDept    StepCode = "supt_dept"
	StepDeptManager StepCode = "dept_manager"
	StepSrManager   StepCode = "sr_manager"
	StepHOO         StepCode = "hoo"
)

var AllSteps = []StepCode{
	StepITCheck, StepITManager, StepSuptDept, StepDeptManager, StepSrManager, StepHOO,
}

func (c StepCode) Order() int {
	for i, s := range AllSteps {
		if s == c {
			return i + 1
		}
	}
	return 0
}

func (c StepCode) Valid() bool { return c.Order() != 0 }

func (c StepCode) IsDepartmentScoped() bool {
	return c == StepSuptDept || c == StepDeptManager || c == StepSrManager
}

func (c StepCode) Label() string {
	switch c {
	case StepITCheck:
		return "Pemeriksa Staff IT"
	case StepITManager:
		return "Manager IT"
	case StepSuptDept:
		return "Supt. Dept"
	case StepDeptManager:
		return "Manager Dept"
	case StepSrManager:
		return "Senior Manager"
	case StepHOO:
		return "Head of Operation"
	}
	return string(c)
}

type StepStatus string

const (
	StepWaiting  StepStatus = "waiting"
	StepActive   StepStatus = "active"
	StepApproved StepStatus = "approved"
	StepRejected StepStatus = "rejected"
	StepSkipped  StepStatus = "skipped"
)

type RequestStatus string

const (
	RequestInReview RequestStatus = "in_review"
	RequestApproved RequestStatus = "approved"
	RequestRejected RequestStatus = "rejected"
)

type Route struct {
	ID                 string    `json:"id"`
	Code               StepCode  `json:"code"`
	Label              string    `json:"label"`
	Departemen         string    `json:"departemen"`
	ApproverEmployeeID string    `json:"approverEmployeeId"`
	ApproverNama       string    `json:"approverNama,omitempty"`
	ApproverJabatan    string    `json:"approverJabatan,omitempty"`
	ApproverActive     bool      `json:"approverActive"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type StepApprover struct {
	EmployeeID string `json:"employeeId"`
	Nama       string `json:"nama"`
	Jabatan    string `json:"jabatan"`
}

type Step struct {
	ID        string     `json:"id"`
	RequestID string     `json:"requestId"`
	Round     int        `json:"round"` //naik 1 tingkat setiap approval
	Code      StepCode   `json:"code"`
	Label     string     `json:"label"`
	StepOrder int        `json:"stepOrder"`
	Status    StepStatus `json:"status"`

	Approvers []StepApprover `json:"approvers"`

	ActedByEmployeeID *string    `json:"actedByEmployeeId,omitempty"`
	ActedByUserID     *string    `json:"actedByUserId,omitempty"`
	ActedByNama       string     `json:"actedByName,omitempty"`
	ActedByJabatan    string     `json:"actedByJabatan,omitempty"`
	ActedAt           *time.Time `json:"actedAt,omitempty"`
	Note              string     `json:"note,omitempty"`

	CarriedOver bool `json:"carriedOver"`

	CreatedAt time.Time `json:"createdAt"`
}

func (s Step) CanAct(employeeID string) bool {
	if s.Status != StepActive || employeeID == "" {
		return false
	}
	for _, a := range s.Approvers {
		if a.EmployeeID == employeeID {
			return true
		}
	}
	return false
}

type Progress struct {
	RequestID   string        `json:"requestId"`
	Status      RequestStatus `json:"status"`
	Round       int           `json:"round"`
	CurrentStep int           `json:"currentStep"`
	Steps       []Step        `json:"steps"`
	CanAct      bool          `json:"canAct"`
}

func (p Progress) Current() *Step {
	for i := range p.Steps {
		if p.Steps[i].Status == StepActive {
			return &p.Steps[i]
		}
	}
	return nil
}

type ApproveInput struct {
	Note string `json:"note"`
}

type RejectInput struct {
	Note string `json:"note" binding:"required"`
}

type RouteInput struct {
	Code               StepCode `json:"code" binding:"required"`
	Departemen         string   `json:"departemen"`
	ApproverEmployeeID string   `json:"approverEmployeeId" binding:"required"`
}

func (in RouteInput) Validate() string {
	switch {
	case !in.Code.Valid():
		return "Kode langkah tidak diketahui"
	case in.Code.IsDepartmentScoped() && in.Departemen == "":
		return "departemen wajib diisi untuk langkah " + in.Code.Label()
	case !in.Code.IsDepartmentScoped() && in.Departemen != "":
		return "langkah " + in.Code.Label() + " berlaku global, departemen harus kosong"
	}
	return ""
}

// Tanda tangan

const (
	sigPNGPrefix = "data:image/png;base64,"
	sigMaxBody   = 80 * 1024
	sigMaxBytes  = 50 * 1024
	sigMaxW      = 600
	sigMaxH      = 300
)

var (
	ErrNoSignature = errors.New("tanda tangan belum di unggah")
	pngMagic       = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
)

type SignatureInput struct {
	Image string `json:"image"`
}

type SignatureResponse struct {
	Image string `json:"image"`
	Has   bool   `json:"has"`
}

func (in SignatureInput) Validate() error {
	if len(in.Image) > sigMaxBody {
		return errors.New("ukuran gambar terlalu besar (maksimal 100KB)")
	}
	if !strings.HasPrefix(in.Image, sigPNGPrefix) {
		return errors.New("format harus png")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(in.Image, sigPNGPrefix))
	if err != nil {
		return errors.New("gambar tidak valid")
	}
	if len(raw) > sigMaxBytes {
		return errors.New("ukuran gambar terlalu besar (maksimal 100KB)")
	}
	if len(raw) < 24 || !bytes.Equal(raw[:8], pngMagic) || string(raw[12:16]) != "IHDR" {
		return errors.New("file bukan png yang valid")
	}

	w := binary.BigEndian.Uint32(raw[16:20])
	h := binary.BigEndian.Uint32(raw[20:24])
	if w == 0 || h == 0 || w > sigMaxW || h > sigMaxH {
		return errors.New("dimensi gambar maksimal 600x300 piksel")
	}
	return nil
}
