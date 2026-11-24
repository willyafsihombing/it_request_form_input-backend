package notify

import (
	"bytes"
	"fmt"
	"html/template"
)

type mailData struct {
	Heading    string
	Greeting   string
	Intro      string
	FormNumber string
	Title      string
	Requester  string
	Department string
	StepLabel  string
	ActedBy    string
	NoteLabel  string
	Note       string
	URL        string
	ButtonText string
	Accent     string
}

var mailTpl = template.Must(template.New("mail").Parse(`<!DOCTYPE html>
<html><body style="margin:0;padding:0;background:#f1f5f9;font-family:Segoe UI,Arial,sans-serif;color:#0f172a;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f1f5f9;padding:24px 0;">
<tr><td align="center">
<table role="presentation" width="600" cellpadding="0" cellspacing="0" style="background:#ffffff;border:1px solid #e2e8f0;">
  <tr><td style="padding:20px 28px;border-bottom:3px solid {{.Accent}};">
    <div style="font-size:12px;color:#64748b;">ITR Platform · PT. Unggul Dinamika Utama</div>
    <div style="font-size:20px;font-weight:bold;color:{{.Accent}};margin-top:4px;">{{.Heading}}</div>
  </td></tr>
  <tr><td style="padding:24px 28px;font-size:14px;line-height:1.6;">
    <p style="margin:0 0 12px;">{{.Greeting}}</p>
    <p style="margin:0 0 16px;">{{.Intro}}</p>
    <table role="presentation" cellpadding="6" cellspacing="0" style="font-size:13px;border-collapse:collapse;width:100%;">
      <tr><td style="color:#64748b;width:150px;border-bottom:1px solid #f1f5f9;">No. Form</td><td style="border-bottom:1px solid #f1f5f9;"><b>{{.FormNumber}}</b></td></tr>
      <tr><td style="color:#64748b;border-bottom:1px solid #f1f5f9;">Judul</td><td style="border-bottom:1px solid #f1f5f9;">{{.Title}}</td></tr>
      <tr><td style="color:#64748b;border-bottom:1px solid #f1f5f9;">Pemohon</td><td style="border-bottom:1px solid #f1f5f9;">{{.Requester}}</td></tr>
      {{if .Department}}<tr><td style="color:#64748b;border-bottom:1px solid #f1f5f9;">Departemen</td><td style="border-bottom:1px solid #f1f5f9;">{{.Department}}</td></tr>{{end}}
      {{if .StepLabel}}<tr><td style="color:#64748b;border-bottom:1px solid #f1f5f9;">Langkah</td><td style="border-bottom:1px solid #f1f5f9;">{{.StepLabel}}</td></tr>{{end}}
      {{if .ActedBy}}<tr><td style="color:#64748b;border-bottom:1px solid #f1f5f9;">Oleh</td><td style="border-bottom:1px solid #f1f5f9;">{{.ActedBy}}</td></tr>{{end}}
    </table>
    {{if .Note}}<div style="margin-top:16px;padding:10px 14px;background:#f8fafc;border-left:3px solid {{.Accent}};font-size:13px;">
      <div style="color:#64748b;font-size:12px;">{{.NoteLabel}}</div><div style="white-space:pre-line;">{{.Note}}</div></div>{{end}}
    <table role="presentation" cellpadding="0" cellspacing="0" style="margin-top:24px;"><tr>
      <td style="background:{{.Accent}};border-radius:4px;">
        <a href="{{.URL}}" style="display:inline-block;padding:10px 22px;color:#ffffff;text-decoration:none;font-weight:bold;font-size:14px;">{{.ButtonText}}</a>
      </td></tr></table>
    <p style="margin:16px 0 0;font-size:12px;color:#64748b;">Jika tombol tidak berfungsi, salin tautan ini ke browser:<br>{{.URL}}</p>
  </td></tr>
  <tr><td style="padding:14px 28px;background:#f8fafc;font-size:11px;color:#94a3b8;">
    Email ini dikirim otomatis oleh sistem, mohon tidak membalas. Tautan hanya bisa dibuka dari jaringan kantor atau VPN.
  </td></tr>
</table>
</td></tr></table>
</body></html>`))

func render(d mailData) (string, error) {
	var buf bytes.Buffer
	if err := mailTpl.Execute(&buf, d); err != nil {
		return "", fmt.Errorf("render email: %w", err)
	}
	return buf.String(), nil
}

func subjectFor(ev Event, form, title string) string {
	switch ev {
	case EventStepActive:
		return fmt.Sprintf("[ITR] Menunggu persetujuan Anda – %s – %s", form, title)
	case EventRejected:
		return fmt.Sprintf("[ITR] Ditolak – %s – %s", form, title)
	case EventApproved:
		return fmt.Sprintf("[ITR] Disetujui – %s – %s", form, title)
	}
	return "[ITR] Pemberitahuan – " + form
}
