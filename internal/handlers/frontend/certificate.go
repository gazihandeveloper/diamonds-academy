package frontend

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"os"

	"github.com/jung-kurt/gofpdf"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/diamondsacademy/diamonds/internal/session"
)

const certTemplate = "web/static/certificate-template.png"

func (h *Handler) Certificate(w http.ResponseWriter, r *http.Request) {
	uid := h.SM.GetInt64(r.Context(), session.KeyUserID)
	all, _ := h.Days.List(r.Context())
	published := 0
	for _, d := range all {
		if d.Published {
			published++
		}
	}
	completed, _ := h.Progress.CompletedDays(r.Context(), uid)
	completedCount := 0
	for _, v := range completed {
		if v {
			completedCount++
		}
	}

	if published == 0 || completedCount < published {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	name := h.SM.GetString(r.Context(), "name")
	if name == "" {
		name = "Katılımcı"
	}

	bg := certBackground()
	img := drawNameOnImage(bg, name)

	var imgBuf bytes.Buffer
	_ = png.Encode(&imgBuf, img)

	pdf := gofpdf.New("L", "mm", "A4", "")
	wPDF, hPDF := pdf.GetPageSize()
	pdf.AddPage()

	opt := gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
	reader := bytes.NewReader(imgBuf.Bytes())
	pdf.RegisterImageOptionsReader("bg", opt, reader)
	pdf.ImageOptions("bg", 0, 0, wPDF, hPDF, false, opt, 0, "")

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename=sertifika.pdf")
	_ = pdf.Output(w)
}

func certBackground() image.Image {
	tmpl, err := loadCertImage(certTemplate)
	if err != nil {
		tmpl = defaultTemplate()
	}
	return tmpl
}

func drawNameOnImage(src image.Image, name string) *image.RGBA {
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, src, image.Point{}, draw.Over)

	face, err := loadGoFont(52)
	if err != nil {
		return dst
	}

	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	y := int(float64(bounds.Dy()) * 0.62)
	drawCenteredText(dst, name, y, black, face)

	return dst
}

func loadCertImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func loadGoFont(size float64) (font.Face, error) {
	f, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(f, &opentype.FaceOptions{
		Size: size,
		DPI:  72,
	})
}

func drawCenteredText(dst *image.RGBA, text string, y int, c color.Color, face font.Face) {
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(c),
		Face: face,
	}
	tw := d.MeasureString(text).Round()
	x := (dst.Bounds().Dx() - tw) / 2
	if x < 0 {
		x = 0
	}
	d.Dot = fixed.P(x, y)
	d.DrawString(text)
}

func defaultTemplate() image.Image {
	w, h := 1200, 850
	dst := image.NewRGBA(image.Rect(0, 0, w, h))

	bg := color.RGBA{R: 8, G: 8, B: 16, A: 255}
	draw.Draw(dst, dst.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	borderColor := color.RGBA{R: 168, G: 85, B: 247, A: 255}
	margin := 50
	drawRect(dst, margin, margin, w-margin*2, h-margin*2, borderColor)
	drawRect(dst, margin+12, margin+12, w-(margin+12)*2, h-(margin+12)*2, borderColor)

	cornerSize := 14
	for i := 0; i < cornerSize; i++ {
		d := color.RGBA{R: 168, G: 85, B: 247, A: uint8(180 - i*10)}
		drawRect(dst, margin-i, margin-i, 1, 1, d)
		drawRect(dst, w-margin+i, margin-i, 1, 1, d)
		drawRect(dst, margin-i, h-margin+i, 1, 1, d)
		drawRect(dst, w-margin+i, h-margin+i, 1, 1, d)
	}

	return dst
}

func drawRect(dst *image.RGBA, x, y, w, h int, c color.Color) {
	for px := x; px < x+w && px < dst.Bounds().Dx(); px++ {
		for py := y; py < y+h && py < dst.Bounds().Dy(); py++ {
			dst.Set(px, py, c)
		}
	}
}
