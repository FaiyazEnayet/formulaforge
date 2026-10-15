// Package gif renders docs/demo.gif: the PHY 131 dependency graph
// assembling itself. Reproducible: real sheet data, real graph
// algorithms. Phases:
//  1. title card,
//  2. waves appear in study order with edges,
//  3. the minimum-memorize set lights up in gold with the coverage stat.
//
// Frames are drawn with fogleman/gg on a dark background and encoded
// with the standard library gif encoder.
package gif

import (
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"math"
	"os"

	"github.com/fogleman/gg"

	"github.com/FaiyazEnayet/formulaforge/internal/graph"
	"github.com/FaiyazEnayet/formulaforge/internal/models"
)

const (
	W        = 640
	H        = 420
	fps      = 10
	titleN   = 12
	perWave  = 10
	finaleN  = 25
	fontSize = 13
)

var (
	bgColor     = color.RGBA{0x0f, 0x14, 0x20, 0xff}
	nodeColor   = color.RGBA{0x7f, 0xb3, 0xff, 0xff}
	edgeColor   = color.RGBA{0x3b, 0x50, 0x7c, 0xff}
	dimColor    = color.RGBA{0x2c, 0x3a, 0x55, 0xff}
	goldColor   = color.RGBA{0xff, 0xd1, 0x66, 0xff}
	textColor   = color.RGBA{0xe8, 0xec, 0xf4, 0xff}
	subColor    = color.RGBA{0x9a, 0xa7, 0xc2, 0xff}
	outlineDark = color.RGBA{0x0f, 0x14, 0x20, 0xff}
	outlineLite = color.RGBA{0xff, 0xff, 0xff, 0xff}
)

var fontPaths = []string{
	"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
	"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	"/usr/share/fonts/TTF/DejaVuSans-Bold.ttf",
	"/usr/share/fonts/TTF/DejaVuSans.ttf",
}

func loadFont(dc *gg.Context, size float64) {
	for _, p := range fontPaths {
		if _, err := os.Stat(p); err == nil {
			_ = dc.LoadFontFace(p, size)
			return
		}
	}
	// last resort: gg ships no default TTF; skip text rather than crash
}

// Build renders the demo GIF for the given sheet and writes it to out.
func Build(sheetName, out string) error {
	sheet, err := models.LoadSheetByName(sheetName)
	if err != nil {
		return err
	}
	g := graph.New(sheet)
	if err := g.AssertAcyclic(); err != nil {
		return err
	}
	waves, err := g.Waves()
	if err != nil {
		return err
	}
	order, err := g.TopoOrder()
	if err != nil {
		return err
	}

	// layered layout in data coords, mapped to pixels
	xs, ys := map[string]float64{}, map[string]float64{}
	maxX := 0.0
	for lv, wave := range waves {
		for j, nid := range wave {
			x := (float64(j) - float64(len(wave)-1)/2) * 1.7
			xs[nid] = x
			ys[nid] = float64(-lv)
			if math.Abs(x) > maxX {
				maxX = math.Abs(x)
			}
		}
	}
	depth := float64(len(waves))
	px := func(x float64) float64 { return W/2 + x/(maxX+1.2)*(W/2-50) }
	py := func(y float64) float64 { return 70 + (0-y)/(depth+0.4)*(H-110) }

	minSet := map[string]bool{}
	for _, id := range g.MinMemorizeSet(nil) {
		minSet[id] = true
	}
	type edge struct{ a, b string }
	var edges []edge
	for _, n := range g.Nodes {
		for _, p := range g.Prereqs[n] {
			edges = append(edges, edge{p, n})
		}
	}

	waveStart := make([]int, len(waves)+1)
	for i := range waveStart {
		waveStart[i] = titleN + i*perWave
	}
	finaleStart := waveStart[len(waves)]
	total := finaleStart + finaleN

	visibleAt := func(f int) map[string]bool {
		vis := map[string]bool{}
		if f < titleN {
			return vis
		}
		for i := 0; i < len(waves); i++ {
			if f >= waveStart[i] {
				for _, nid := range waves[i] {
					vis[nid] = true
				}
			}
		}
		return vis
	}

	var frames []*image.Paletted
	var delays []int
	for f := 0; f < total; f++ {
		dc := gg.NewContext(W, H)
		loadFont(dc, 30)
		dc.SetColor(bgColor)
		dc.Clear()

		if f < titleN {
			dc.SetColor(nodeColor)
			loadFont(dc, 30)
			dc.DrawStringAnchored("formulaforge", W/2, H/2-20, 0.5, 0.5)
			loadFont(dc, fontSize)
			dc.SetColor(subColor)
			dc.DrawStringAnchored(fmt.Sprintf("%s — every formula, mapped by what it builds on", sheet.Course), W/2, H/2+25, 0.5, 0.5)
		} else {
			vis := visibleAt(f)
			finale := f >= finaleStart
			// edges
			for _, e := range edges {
				if vis[e.a] && vis[e.b] {
					dc.SetColor(edgeColor)
					dc.SetLineWidth(1)
					dc.DrawLine(px(xs[e.a]), py(ys[e.a]), px(xs[e.b]), py(ys[e.b]))
					dc.Stroke()
				}
			}
			// nodes
			for nid := range vis {
				lv := g.Leverage(nid)
				r := 5 + 2.6*math.Sqrt(float64(lv))
				x, y := px(xs[nid]), py(ys[nid])
				switch {
				case finale && minSet[nid]:
					dc.SetColor(goldColor)
				case finale:
					dc.SetColor(dimColor)
				default:
					dc.SetColor(nodeColor)
				}
				dc.DrawCircle(x, y, r)
				dc.Fill()
				if finale && minSet[nid] {
					dc.SetColor(outlineLite)
				} else {
					dc.SetColor(outlineDark)
				}
				dc.SetLineWidth(1.2)
				dc.DrawCircle(x, y, r)
				dc.Stroke()
			}
			loadFont(dc, fontSize)
			if !finale {
				w := 0
				for i := 0; i < len(waves); i++ {
					if f >= waveStart[i] {
						w = i
					}
				}
				dc.SetColor(textColor)
				dc.DrawStringAnchored(fmt.Sprintf("study wave %d of %d", w+1, len(waves)), W/2, 32, 0.5, 0.5)
			} else {
				dc.SetColor(goldColor)
				dc.DrawStringAnchored(fmt.Sprintf("memorize %d, derive the other %d", len(minSet), len(order)-len(minSet)), W/2, 28, 0.5, 0.5)
				loadFont(dc, 10)
				dc.SetColor(subColor)
				dc.DrawStringAnchored("minimum-memorize set (greedy set cover)", W/2, 50, 0.5, 0.5)
			}
		}

		// quantize to palette
		img := dc.Image()
		paletted := image.NewPaletted(img.Bounds(), paletteFor())
		ditherCopy(paletted, img)
		frames = append(frames, paletted)
		delays = append(delays, 100/fps)
	}

	fh, err := os.Create(out)
	if err != nil {
		return err
	}
	defer fh.Close()
	return gif.EncodeAll(fh, &gif.GIF{
		Image: frames,
		Delay: delays,
	})
}

// paletteFor returns a fixed 256-color palette covering our drawing colors.
func paletteFor() color.Palette {
	p := color.Palette{}
	// grayscale ramp for antialiasing
	for i := 0; i < 216; i++ {
		v := uint8(i * 255 / 215)
		p = append(p, color.RGBA{v, v, v, 0xff})
	}
	for _, c := range []color.Color{bgColor, nodeColor, edgeColor, dimColor, goldColor, textColor, subColor, outlineDark, outlineLite} {
		p = append(p, c)
	}
	for len(p) < 256 {
		p = append(p, color.RGBA{0, 0, 0, 0xff})
	}
	return p[:256]
}

// ditherCopy does a nearest-color copy (no dithering; our art is flat).
func ditherCopy(dst *image.Paletted, src image.Image) {
	b := src.Bounds()
	pal := dst.Palette
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.SetColorIndex(x, y, uint8(pal.Index(src.At(x, y))))
		}
	}
}
