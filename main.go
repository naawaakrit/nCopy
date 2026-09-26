// seqcopy - โปรแกรมคัดลอกไฟล์ทีละไฟล์เรียงตามตัวอักษร (คล้าย TeraCopy)
// เขียนด้วย Go + Fyne สำหรับ Linux
package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

func main() {
	a := app.New()
	w := a.NewWindow("nCopy - คัดลอกไฟล์เรียงตามตัวอักษร")
	w.Resize(fyne.NewSize(720, 640))

	ap := &app_{fyneApp: a, win: w, ctrl: newController(), preserveMetadata: true, maxRetry: 3}

	w.SetOnDropped(func(pos fyne.Position, uris []fyne.URI) {
		added := false
		for _, u := range uris {
			if u.Scheme() == "file" || u.Scheme() == "" {
				path := u.Path()
				if path != "" {
					ap.sources = append(ap.sources, path)
					added = true
				}
			}
		}
		if added {
			ap.sourceList.Refresh()
			ap.rebuildQueue()
		}
	})

	w.SetContent(ap.buildUI())
	w.ShowAndRun()
}
