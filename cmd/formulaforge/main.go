// Command formulaforge is the STEM formula-sheet studio CLI.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/FaiyazEnayet/formulaforge/internal/anki"
	"github.com/FaiyazEnayet/formulaforge/internal/dashboard"
	"github.com/FaiyazEnayet/formulaforge/internal/gif"
	"github.com/FaiyazEnayet/formulaforge/internal/graph"
	"github.com/FaiyazEnayet/formulaforge/internal/models"
	"github.com/FaiyazEnayet/formulaforge/internal/practice"
	"github.com/FaiyazEnayet/formulaforge/internal/render"
	"github.com/FaiyazEnayet/formulaforge/internal/search"
)

var version = "0.1.1"

func mustSheet(name string) *models.Sheet {
	s, err := models.LoadSheetByName(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	return s
}

func mustGraph(s *models.Sheet) *graph.Graph {
	g := graph.New(s)
	if err := g.AssertAcyclic(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	return g
}

func main() {
	root := &cobra.Command{
		Use:     "formulaforge",
		Short:   "Formula sheet studio for STEM students",
		Version: version,
	}

	root.AddCommand(&cobra.Command{
		Use:   "sheets",
		Short: "List available sheets",
		Run: func(cmd *cobra.Command, args []string) {
			for _, name := range models.ListSheets() {
				s := mustSheet(name)
				fmt.Printf("%-10s %-8s %s (%d formulas)\n", name, s.Course, s.Title, len(s.Formulas))
			}
		},
	})

	buildCmd := &cobra.Command{
		Use:   "build <sheet>",
		Short: "Render a sheet to printable HTML",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			sheet := mustSheet(args[0])
			out, _ := cmd.Flags().GetString("output")
			if out == "" {
				out = args[0] + ".html"
			}
			page, err := render.HTML(sheet)
			if err != nil {
				fatal(err)
			}
			if err := os.WriteFile(out, []byte(page), 0o644); err != nil {
				fatal(err)
			}
			fmt.Printf("built %s (%d formulas)\n", out, len(sheet.Formulas))
		},
	}
	buildCmd.Flags().StringP("output", "o", "", "output file")
	root.AddCommand(buildCmd)

	graphCmd := &cobra.Command{
		Use:   "graph <sheet>",
		Short: "Dependency graph stats and centrality",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			sheet := mustSheet(args[0])
			g := mustGraph(sheet)
			st, err := g.Stats()
			if err != nil {
				fatal(err)
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			out, _ := cmd.Flags().GetString("output")
			if asJSON {
				levels, _ := g.Levels()
				nodes := make([]map[string]any, 0, len(sheet.Formulas))
				for i := range sheet.Formulas {
					f := &sheet.Formulas[i]
					nodes = append(nodes, map[string]any{
						"id": f.ID, "name": f.Name, "topic": f.Topic,
						"level": levels[f.ID], "leverage": g.Leverage(f.ID),
					})
				}
				var edges []map[string]string
				for _, n := range g.Nodes {
					for _, p := range g.Prereqs[n] {
						edges = append(edges, map[string]string{"from": p, "to": n})
					}
				}
				top := make([]map[string]any, 0, len(st.TopLeverage))
				for _, e := range st.TopLeverage {
					top = append(top, map[string]any{"id": e.ID, "unlocks": e.Leverage})
				}
				payload := map[string]any{
					"sheet": args[0], "nodes": nodes, "edges": edges,
					"stats": map[string]any{
						"nodes": st.Nodes, "edges": st.Edges, "depth": st.Depth,
						"waves": st.Waves, "roots": st.Roots, "leaves": st.Leaves,
						"min_memorize_set": st.MinMemorizeSet,
						"min_memorize_size": len(st.MinMemorizeSet),
					},
					"top_leverage": top,
				}
				if out == "" {
					out = args[0] + ".graph.json"
				}
				data, _ := json.MarshalIndent(payload, "", "  ")
				if err := os.WriteFile(out, data, 0o644); err != nil {
					fatal(err)
				}
				fmt.Printf("wrote %s\n", out)
				return
			}
			fmt.Printf("%s %s\n", sheet.Course, sheet.Title)
			fmt.Printf("  formulas:   %d\n", st.Nodes)
			fmt.Printf("  derivations:%d\n", st.Edges)
			fmt.Printf("  study waves:%d (longest chain: %d)\n", st.Waves, st.Depth)
			fmt.Printf("  roots: %d, leaves: %d\n", st.Roots, st.Leaves)
			fmt.Println("  highest leverage (unlocks the most):")
			for _, e := range st.TopLeverage {
				f, _ := sheet.ByID(e.ID)
				fmt.Printf("    %s: %d\n", f.Name, e.Leverage)
			}
			fmt.Printf("  minimum memorize set (%d):\n", len(st.MinMemorizeSet))
			for _, id := range st.MinMemorizeSet {
				f, _ := sheet.ByID(id)
				fmt.Printf("    - %s\n", f.Name)
			}
			if tg, _ := cmd.Flags().GetString("target"); tg != "" {
				var targets []string
				for _, t := range strings.Split(tg, ",") {
					if t = strings.TrimSpace(t); t != "" {
						targets = append(targets, t)
					}
				}
				ms, err := g.MinMemorizeFor(targets, nil)
				if err != nil {
					fatal(err)
				}
				fmt.Printf("  smallest set covering targets (%d):\n", len(ms))
				for _, id := range ms {
					f, _ := sheet.ByID(id)
					fmt.Printf("    - %s\n", f.Name)
				}
			}
		},
	}
	graphCmd.Flags().Bool("json", false, "emit JSON")
	graphCmd.Flags().String("target", "", "comma-separated formula ids: print the smallest memorize set covering just those")
	graphCmd.Flags().StringP("output", "o", "", "output file for --json")
	root.AddCommand(graphCmd)

	root.AddCommand(&cobra.Command{
		Use:   "study-order <sheet>",
		Short: "Topological study order in waves",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			sheet := mustSheet(args[0])
			g := mustGraph(sheet)
			waves, err := g.Waves()
			if err != nil {
				fatal(err)
			}
			for i, wave := range waves {
				fmt.Printf("Wave %d:\n", i+1)
				for _, id := range wave {
					f, _ := sheet.ByID(id)
					fmt.Printf("  - %s\n", f.Name)
				}
			}
		},
	})

	exportCmd := &cobra.Command{
		Use:   "export <sheet>",
		Short: "Export a sheet (html, pdf, tex, anki)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			sheet := mustSheet(args[0])
			format, _ := cmd.Flags().GetString("format")
			out, _ := cmd.Flags().GetString("output")
			if out == "" {
				ext := map[string]string{"html": "html", "pdf": "pdf", "tex": "tex", "anki": "tsv"}[format]
				out = args[0] + "." + ext
			}
			switch format {
			case "html":
				page, err := render.HTML(sheet)
				if err != nil {
					fatal(err)
				}
				if err := os.WriteFile(out, []byte(page), 0o644); err != nil {
					fatal(err)
				}
			case "tex":
				doc, err := render.Latex(sheet)
				if err != nil {
					fatal(err)
				}
				if err := os.WriteFile(out, []byte(doc), 0o644); err != nil {
					fatal(err)
				}
				fmt.Println("  compile with: pdflatex", out)
			case "pdf":
				info, err := render.PDF(sheet, out)
				if err != nil {
					fatal(err)
				}
				fmt.Printf("  pages: %d — %s\n", info.Pages, info.Note)
			case "anki":
				rows, err := anki.ExportTSV(sheet, out)
				if err != nil {
					fatal(err)
				}
				fmt.Printf("  rows: %d (Front/Back/Topic — import via Anki > File > Import)\n", rows)
			default:
				fatal(fmt.Errorf("unknown format: %s (html, pdf, tex, anki)", format))
			}
			fmt.Printf("exported %s\n", out)
		},
	}
	exportCmd.Flags().String("format", "", "export format")
	_ = exportCmd.MarkFlagRequired("format")
	exportCmd.Flags().StringP("output", "o", "", "output file")
	root.AddCommand(exportCmd)

	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across sheets",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			sheets, err := models.LoadAll()
			if err != nil {
				fatal(err)
			}
			only, _ := cmd.Flags().GetString("sheet")
			if only != "" {
				s, ok := sheets[only]
				if !ok {
					fatal(fmt.Errorf("no sheet named %q", only))
				}
				sheets = map[string]*models.Sheet{only: s}
			}
			top, _ := cmd.Flags().GetInt("top")
			hits := search.Search(sheets, args[0], top)
			if len(hits) == 0 {
				fmt.Println("no matches")
				return
			}
			for _, h := range hits {
				fmt.Printf("[%s] %s (%s)  score=%.1f\n", h.Sheet, h.Form.Name, h.Form.Topic, h.Score)
			}
		},
	}
	searchCmd.Flags().String("sheet", "", "restrict to one sheet")
	searchCmd.Flags().Int("top", 10, "max results")
	root.AddCommand(searchCmd)

	practiceCmd := &cobra.Command{
		Use:   "practice <sheet>",
		Short: "Quiz yourself on derivations",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			sheet := mustSheet(args[0])
			g := mustGraph(sheet)
			mode, _ := cmd.Flags().GetString("mode")
			n, _ := cmd.Flags().GetInt("n")
			seed, _ := cmd.Flags().GetInt64("seed")
			var questions []practice.Question
			switch mode {
			case "derive":
				questions = practice.GenDeriveQuestions(g, n, seed)
			case "order":
				questions = practice.GenOrderQuestions(g, n, seed)
			case "state":
				questions = practice.GenStateQuestions(g, n, seed)
			default:
				fatal(fmt.Errorf("unknown mode: %s", mode))
			}
			if len(questions) == 0 {
				fmt.Println("no questions available for this mode")
				return
			}
			nameOf := map[string]string{}
			for i := range sheet.Formulas {
				nameOf[sheet.Formulas[i].ID] = sheet.Formulas[i].Name
			}
			in := bufio.NewReader(os.Stdin)
			ask := func(q practice.Question) []string {
				switch q.Type {
				case practice.TypeDerive:
					fmt.Printf("\nDERIVE: '%s' builds on which formulas?\n", nameOf[q.Formula])
					fmt.Println("  (comma-separated ids; available: " + strings.Join(sortedKeys(g.Nodes), ", ") + ")")
					line := readLine(in)
					return strings.Split(line, ",")
				case practice.TypeOrder:
					fmt.Printf("\nORDER: which must be learned first?\n  1) %s\n  2) %s\n", nameOf[q.First], nameOf[q.Second])
					fmt.Print("1 or 2 > ")
					pick := strings.TrimSpace(readLine(in))
					if pick == "1" {
						return []string{q.First}
					}
					return []string{q.Second}
				default:
					f, _ := sheet.ByID(q.Formula)
					fmt.Printf("\nSTATE: %s\n  %s\n", f.Name, f.Description)
					fmt.Print("did you recall it? (y/n) > ")
					return []string{readLine(in)}
				}
			}
			summary := practice.RunSession(g, questions, ask)
			fmt.Printf("\nscore: %d/%d\n", summary.Correct, summary.Total)
			for _, r := range summary.Results {
				if !r.OK && r.Question.Type == practice.TypeDerive {
					truth := make([]string, 0, len(r.Derive.Truth))
					for _, t := range r.Derive.Truth {
						truth = append(truth, nameOf[t])
					}
					fmt.Printf("  miss: %s -> %s\n", nameOf[r.Question.Formula], strings.Join(truth, ", "))
				}
			}
		},
	}
	practiceCmd.Flags().String("mode", "derive", "quiz mode: derive, order, state")
	practiceCmd.Flags().Int("n", 10, "number of questions")
	practiceCmd.Flags().Int64("seed", 7, "random seed")
	root.AddCommand(practiceCmd)

	dashCmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Generate the interactive HTML dashboard",
		Run: func(cmd *cobra.Command, args []string) {
			out, _ := cmd.Flags().GetString("output")
			if out == "" {
				out = "docs/dashboard.html"
			}
			if err := dashboard.Build(out); err != nil {
				fatal(err)
			}
			fi, _ := os.Stat(out)
			fmt.Printf("dashboard -> %s (%.1f MB)\n", out, float64(fi.Size())/1e6)
		},
	}
	dashCmd.Flags().StringP("output", "o", "", "output file")
	root.AddCommand(dashCmd)

	showcaseCmd := &cobra.Command{
		Use:   "export-showcase",
		Short: "Regenerate scripts/showcase-data.json from the engine",
		Run: func(cmd *cobra.Command, args []string) {
			sheet, _ := cmd.Flags().GetString("sheet")
			out, _ := cmd.Flags().GetString("output")
			if out == "" {
				out = "scripts/showcase-data.json"
			}
			exp, err := dashboard.ExportShowcase(sheet)
			if err != nil {
				fatal(err)
			}
			if err := dashboard.WriteShowcase(out, out, exp); err != nil {
				fatal(err)
			}
			fmt.Printf("showcase -> %s (sheet %s, source %s, curve %s)\n", out, exp.Provenance.SourceFile, exp.Provenance.SourceSHA256[:12], exp.Provenance.CurveSHA256[:12])
		},
	}
	showcaseCmd.Flags().String("sheet", "phy131", "sheet to export")
	showcaseCmd.Flags().StringP("output", "o", "", "output file")
	root.AddCommand(showcaseCmd)

	gifCmd := &cobra.Command{
		Use:   "demogif",
		Short: "Render the animated demo GIF",
		Run: func(cmd *cobra.Command, args []string) {
			sheet, _ := cmd.Flags().GetString("sheet")
			out, _ := cmd.Flags().GetString("output")
			if out == "" {
				out = "docs/demo.gif"
			}
			if err := gif.Build(sheet, out); err != nil {
				fatal(err)
			}
			fi, _ := os.Stat(out)
			fmt.Printf("gif -> %s (%.2f MB)\n", out, float64(fi.Size())/1e6)
		},
	}
	gifCmd.Flags().String("sheet", "phy131", "sheet to animate")
	gifCmd.Flags().StringP("output", "o", "", "output file")
	root.AddCommand(gifCmd)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func readLine(in *bufio.Reader) string {
	line, _ := in.ReadString('\n')
	return strings.TrimRight(line, "\r\n")
}

func sortedKeys(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}
