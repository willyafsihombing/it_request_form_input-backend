package onedbsync

import (
	"encoding/json"
	"strings"
)

type Flexstring string

func (f *Flexstring) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		*f = ""
		return nil
	}
	if s[0] == '"' {
		var v string
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		*f = Flexstring(v)
		return nil
	}
	*f = Flexstring(s)
	return nil
}

type EmployeeAPI struct {
	NoNIK          Flexstring `json:"no_nik"`
	DOH            Flexstring `json:"doh"`
	TglAktif       Flexstring `json:"tgl_aktif"`
	Klasifikasi    Flexstring `json:"klasifikasi"`
	TglPermanen    Flexstring `json:"tgl_permanen"`
	TglNonaktif    Flexstring `json:"tgl_nonaktif"`
	NamaLengkap    Flexstring `json:"nama_lengkap"`
	NamaAlias      Flexstring `json:"nama_alias"`
	JK             Flexstring `json:"jk"`
	TglLahir       Flexstring `json:"tgl_lahir"`
	StatNikah      Flexstring `json:"stat_nikah"`
	WN             Flexstring `json:"wn"`
	EmailPribadi   Flexstring `json:"email_pribadi"`
	EmailKantor    Flexstring `json:"email_kantor"`
	HP1            Flexstring `json:"hp_1"`
	Depart         Flexstring `json:"depart"`
	Section        Flexstring `json:"section"`
	Posisi         Flexstring `json:"posisi"`
	Level          Flexstring `json:"level"`
	Lokker         Flexstring `json:"lokker"`
	Roster         Flexstring `json:"roster"`
	Tipe           Flexstring `json:"tipe"`
	NamaPerusahaan Flexstring `json:"nama_perusahaan"`
	Usia           Flexstring `json:"usia"`
	LamaBekerja    Flexstring `json:"lama_bekerja"`
	StatTinggal    Flexstring `json:"stat_tinggal"`
	CreatedAt      Flexstring `json:"created_at"`
	UpdatedAt      Flexstring `json:"updated_at"`
	DeletedAt      Flexstring `json:"deleted_at"`
}

type EmployeeSync struct {
	NIK        string
	Nama       string
	Jabatan    string
	Departemen string
	Level      string
	StaffType  string
	IsActive   bool
}

type EmployeePreview struct {
	NIK        string `json:"nik"`
	Nama       string `json:"nama"`
	Jabatan    string `json:"jabatan"`
	Departemen string `json:"departemen"`
	Level      string `json:"level"`
	StaffType  string `json:"staff_type"`
	IsActive   bool   `json:"is_active"`
}
