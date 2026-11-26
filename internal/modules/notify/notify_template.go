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
<html lang="id">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="color-scheme" content="light only">
  <title>{{.Heading}}</title>
</head>
<body style="margin:0;padding:0;background-color:#f1f5f9;">
  <div style="display:none;max-height:0;overflow:hidden;opacity:0;font-size:1px;line-height:1px;color:#f1f5f9;">
    {{.FormNumber}} – {{.Title}}
  </div>

  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" bgcolor="#f1f5f9">
    <tr>
      <td align="center" style="padding:32px 16px;">

        <table role="presentation" width="600" cellpadding="0" cellspacing="0" border="0"
               style="width:100%;max-width:600px;background-color:#ffffff;border-radius:12px;border:1px solid #e2e8f0;">

          <tr>
            <td height="5" bgcolor="{{.Accent}}" style="height:5px;line-height:5px;font-size:0;background-color:{{.Accent}};border-radius:12px 12px 0 0;">&nbsp;</td>
          </tr>

          <tr>
            <td style="padding:28px 36px 8px 36px;font-family:'Segoe UI',Arial,Helvetica,sans-serif;">
              <div style="font-size:12px;letter-spacing:1px;text-transform:uppercase;color:#64748b;font-weight:600;">
                ITR Platform · PT. Unggul Dinamika Utama
              </div>
              <h1 style="margin:10px 0 0 0;font-size:24px;line-height:1.3;color:#0f172a;font-weight:700;">
                {{.Heading}}
              </h1>
            </td>
          </tr>

          <tr>
            <td style="padding:16px 36px 8px 36px;font-family:'Segoe UI',Arial,Helvetica,sans-serif;font-size:15px;line-height:1.65;color:#334155;">
              <p style="margin:0 0 12px 0;"><strong style="color:#0f172a;">{{.Greeting}}</strong></p>
              <p style="margin:0;">{{.Intro}}</p>
            </td>
          </tr>

          <tr>
            <td style="padding:20px 36px 8px 36px;">
              <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"
                     style="background-color:#f8fafc;border:1px solid #e2e8f0;border-radius:10px;">
                <tr>
                  <td style="padding:6px 20px;font-family:'Segoe UI',Arial,Helvetica,sans-serif;">
                    <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0">
                      <tr>
                        <td width="110" valign="top" style="padding:11px 0;font-size:13px;color:#64748b;border-bottom:1px solid #e2e8f0;">No. Form</td>
                        <td valign="top" style="padding:11px 0;font-size:14px;color:#0f172a;font-weight:700;border-bottom:1px solid #e2e8f0;">{{.FormNumber}}</td>
                      </tr>
                      <tr>
                        <td valign="top" style="padding:11px 0;font-size:13px;color:#64748b;border-bottom:1px solid #e2e8f0;">Judul</td>
                        <td valign="top" style="padding:11px 0;font-size:14px;color:#0f172a;border-bottom:1px solid #e2e8f0;">{{.Title}}</td>
                      </tr>
                      <tr>
                        <td valign="top" style="padding:11px 0;font-size:13px;color:#64748b;border-bottom:1px solid #e2e8f0;">Pemohon</td>
                        <td valign="top" style="padding:11px 0;font-size:14px;color:#0f172a;border-bottom:1px solid #e2e8f0;">{{.Requester}}</td>
                      </tr>
                      <tr>
                        <td valign="top" style="padding:11px 0;font-size:13px;color:#64748b;border-bottom:1px solid #e2e8f0;">Departemen</td>
                        <td valign="top" style="padding:11px 0;font-size:14px;color:#0f172a;border-bottom:1px solid #e2e8f0;">{{.Department}}</td>
                      </tr>
                      <tr>
                        <td valign="top" style="padding:11px 0;font-size:13px;color:#64748b;{{if or .ActedBy .Note}}border-bottom:1px solid #e2e8f0;{{end}}">Langkah</td>
                        <td valign="top" style="padding:11px 0;font-size:14px;{{if or .ActedBy .Note}}border-bottom:1px solid #e2e8f0;{{end}}">
                          <span style="display:inline-block;padding:3px 10px;background-color:#e2e8f0;color:#334155;border-radius:999px;font-size:12px;font-weight:700;">{{.StepLabel}}</span>
                        </td>
                      </tr>
                      {{if .ActedBy}}
                      <tr>
                        <td valign="top" style="padding:11px 0;font-size:13px;color:#64748b;{{if .Note}}border-bottom:1px solid #e2e8f0;{{end}}">Oleh</td>
                        <td valign="top" style="padding:11px 0;font-size:14px;color:#0f172a;{{if .Note}}border-bottom:1px solid #e2e8f0;{{end}}">{{.ActedBy}}</td>
                      </tr>
                      {{end}}
                      {{if .Note}}
                      <tr>
                        <td valign="top" style="padding:11px 0;font-size:13px;color:#64748b;">{{.NoteLabel}}</td>
                        <td valign="top" style="padding:11px 0;font-size:14px;color:#0f172a;white-space:pre-wrap;">{{.Note}}</td>
                      </tr>
                      {{end}}
                    </table>
                  </td>
                </tr>
              </table>
            </td>
          </tr>

          <tr>
            <td align="left" style="padding:24px 36px 8px 36px;">
              <table role="presentation" cellpadding="0" cellspacing="0" border="0">
                <tr>
                  <td align="center" bgcolor="{{.Accent}}" style="background-color:{{.Accent}};border-radius:8px;">
                    <a href="{{.URL}}" target="_blank"
                       style="display:inline-block;padding:14px 32px;font-family:'Segoe UI',Arial,Helvetica,sans-serif;font-size:15px;font-weight:700;line-height:1;color:#ffffff !important;text-decoration:none;border-radius:8px;">
                      <font color="#ffffff"><span style="color:#ffffff;">{{.ButtonText}} &rarr;</span></font>
                    </a>
                  </td>
                </tr>
              </table>
            </td>
          </tr>

          <tr>
            <td style="padding:16px 36px 28px 36px;font-family:'Segoe UI',Arial,Helvetica,sans-serif;font-size:12px;line-height:1.6;color:#64748b;">
              Jika tombol tidak berfungsi, salin tautan ini ke browser:<br>
              <a href="{{.URL}}" style="color:#0369a1;word-break:break-all;text-decoration:underline;">{{.URL}}</a>
            </td>
          </tr>

          <tr>
            <td style="padding:18px 36px;background-color:#f8fafc;border-top:1px solid #e2e8f0;border-radius:0 0 12px 12px;font-family:'Segoe UI',Arial,Helvetica,sans-serif;font-size:12px;line-height:1.6;color:#94a3b8;">
              Email ini dikirim otomatis oleh sistem, mohon tidak membalas.
              Tautan hanya bisa dibuka dari jaringan kantor atau VPN.
            </td>
          </tr>

        </table>

      </td>
    </tr>
  </table>
</body>
</html>`))

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
