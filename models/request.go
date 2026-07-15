package models

import "time"

type Request struct {
	ID               string     `json:"id"`
	FormNumber       string     `json:"formNumber"`
	RequestTitle     string     `json:"requestTitle"`
	EffectiveDate    string     `json:"effectiveDate"`
	ReqCompany       string     `json:"reqCompany"`
	ReqName          string     `json:"reqName"`
	RecCompany       string     `json:"recCompany"`
	RecDept          string     `json:"recDept"`
	RecLocation      string     `json:"recLocation"`
	RecPersonnel     string     `json:"recPersonnel"`
	RecEmpId         string     `json:"recEmpId"`
	RecTitle         string     `json:"recTitle"`
	RecStatus        string     `json:"recStatus"`
	SnAdd            bool       `json:"snAdd"`
	SnChange         bool       `json:"snChange"`
	SnTerminate      bool       `json:"snTerminate"`
	SnLAN            bool       `json:"snLAN"`
	SnVPN            bool       `json:"snVPN"`
	SnEmail          bool       `json:"snEmail"`
	SnFileSharing    bool       `json:"snFileSharing"`
	SnIntranet       bool       `json:"snIntranet"`
	SnOther          string     `json:"snOther"`
	HwAdd            bool       `json:"hwAdd"`
	HwChange         bool       `json:"hwChange"`
	HwTerminate      bool       `json:"hwTerminate"`
	HwPC             bool       `json:"hwPC"`
	HwPrinter        bool       `json:"hwPrinter"`
	HwNotebook       bool       `json:"hwNotebook"`
	HwMSOffice       bool       `json:"hwMSOffice"`
	HwAdobe          bool       `json:"hwAdobe"`
	HwZoom           bool       `json:"hwZoom"`
	HwOther          string     `json:"hwOther"`
	ErpAdd           bool       `json:"erpAdd"`
	ErpChange        bool       `json:"erpChange"`
	ErpTerminate     bool       `json:"erpTerminate"`
	ErpPronto        bool       `json:"erpPronto"`
	ErpSmartMining   bool       `json:"erpSmartMining"`
	ErpPositionId    string     `json:"erpPositionId"`
	ErpDistrict      string     `json:"erpDistrict"`
	ErpRef           string     `json:"erpRef"`
	ErpSignOnId      string     `json:"erpSignOnId"`
	ErpGlobalProfile string     `json:"erpGlobalProfile"`
	AdditionalDesc   string     `json:"additionalDesc"`
	CostCode         string     `json:"costCode"`
	Justification    string     `json:"justification"`
	TechComment      string     `json:"techComment"`
	TechAD           bool       `json:"techAD"`
	TechAW           bool       `json:"techAW"`
	TechCR           bool       `json:"techCR"`
	TechEM           bool       `json:"techEM"`
	TechIP           bool       `json:"techIP"`
	TechEP           bool       `json:"techEP"`
	SigRequester     string     `json:"sigRequester"`
	SigSptDept       string     `json:"sigSptDept"`
	SigDeptMgr       string     `json:"sigDeptMgr"`
	SigSrMgr         string     `json:"sigSrMgr"`
	SigHOO           string     `json:"sigHOO"`
	SigITAdmin       string     `json:"sigITAdmin"`
	SigMSDIMgr       string     `json:"sigMSDIMgr"`
	Status           string     `json:"status"`
	Priority         string     `json:"priority"`
	ItNotes          string     `json:"itNotes"`
	AssignedTo       string     `json:"assignedTo"`
	DeletedAt        *time.Time `json:"deletedAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

// Request list item — field terbatas untuk GET all
type RequestListItem struct {
	ID             string    `json:"id"`
	FormNumber     string    `json:"formNumber"`
	EffectiveDate  string    `json:"effectiveDate"`
	RequestTitle   string    `json:"RequestTitle"`
	ReqName        string    `json:"reqName"`
	RecDept        string    `json:"recDept"`
	RecLocation    string    `json:"recLocation"`
	RecPersonnel   string    `json:"recPersonnel"`
	RecEmpId       string    `json:"recEmpId"`
	HwAdd          bool      `json:"hwAdd"`
	HwChange       bool      `json:"hwChange"`
	HwTerminate    bool      `json:"hwTerminate"`
	SnAdd          bool      `json:"snAdd"`
	SnChange       bool      `json:"snChange"`
	SnTerminate    bool      `json:"snTerminate"`
	ErpAdd         bool      `json:"erpAdd"`
	ErpChange      bool      `json:"erpChange"`
	ErpTerminate   bool      `json:"erpTerminate"`
	HwPC           bool      `json:"hwPC"`
	HwPrinter      bool      `json:"hwPrinter"`
	HwNotebook     bool      `json:"hwNotebook"`
	HwOther        string    `json:"hwOther"`
	AdditionalDesc string    `json:"additionalDesc"`
	Status         string    `json:"status"`
	Priority       string    `json:"priority"`
	AssignedTo     string    `json:"assignedTo"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
