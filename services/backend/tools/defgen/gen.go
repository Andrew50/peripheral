//go:build generate

//go:generate go run gen.go

package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings" // Import strings package
	"text/template"

	"gopkg.in/yaml.v3"
)

// defs holds the raw definitions loaded from YAML.
// Field names are lowerCamelCase and match the YAML keys directly.
type defs struct {
	SecurityFeatures    []string `yaml:"securityFeatures"`
	OhlcvFeatures       []string `yaml:"ohlcvFeatures"`
	FundamentalFeatures []string `yaml:"fundamentalFeatures"`
	Timeframes          []string `yaml:"timeframes"`
	OutputTypes         []string `yaml:"outputTypes"`
	ComparisonOperators []string `yaml:"comparisonOperators"`
	ExprOperators       []string `yaml:"exprOperators"`
	Directions          []string `yaml:"directions"`
}

// templateData holds all the data needed for both templates.
// It includes a combined list for expression columns.
type templateData struct {
	Defs        defs     // Embed the original defs as an exported field
	ExprColumns []string // Combined OHLCV + Fundamental
}

// joinQuote is a template helper function to join a slice of strings
// with commas and wrap each item in double quotes.
func joinQuote(s []string) string {
	quoted := make([]string, len(s))
	for i, item := range s {
		quoted[i] = fmt.Sprintf(`"%s"`, item) // Wrap in quotes
	}
	return strings.Join(quoted, ", ") // Join with ", "
}

// Define template functions
var funcMap = template.FuncMap{
	"joinQuote": joinQuote,
}

// Parse templates with the helper function
var (
	// Template for Go code - assumes template moved to file
	tmplGo = template.Must(template.New("defs_gen.tmpl").Funcs(funcMap).ParseFiles(filepath.Join(scriptDir(), "defs_gen.tmpl")))
	// Template for System Prompt
	tmplPrompt = template.Must(template.New("spec.tmpl").Funcs(funcMap).ParseFiles(filepath.Join(scriptDir(), "spec.tmpl")))
)

func main() {
	base := scriptDir() // Get directory containing this script

	// --- Read YAML Definitions ---
	yamlBytes, err := os.ReadFile(filepath.Join(base, "defs.yaml"))
	must(err)

	var d defs
	must(yaml.Unmarshal(yamlBytes, &d))

	// --- Prepare Data for Templates ---
	data := templateData{
		Defs: d,
		// Combine OHLCV and Fundamental features for the prompt
		ExprColumns: append(append([]string{}, d.OhlcvFeatures...), d.FundamentalFeatures...),
	}

	// --- Generate Go Code (defs_gen.go) ---
	goOut := new(bytes.Buffer)
	// Execute the specific template named "defs_gen.tmpl"
	must(tmplGo.ExecuteTemplate(goOut, "defs_gen.tmpl", data))
	dstGo := filepath.Join(base, "..", "strategies", "defs_gen.go")
	must(os.WriteFile(dstGo, goOut.Bytes(), 0644))
	log.Println("Generated:", dstGo)

	// --- Generate System Prompt (spec.txt) ---
	promptOut := new(bytes.Buffer)
	// Execute the specific template named "spec.tmpl"
	must(tmplPrompt.ExecuteTemplate(promptOut, "spec.tmpl", data))
	// Output path for the prompt, relative to script dir
	dstPrompt := filepath.Join(base, "..", "prompts", "spec.txt")
	// Ensure the target directory exists
	promptDir := filepath.Dir(dstPrompt)
	if _, err := os.Stat(promptDir); os.IsNotExist(err) {
		must(os.MkdirAll(promptDir, 0755))
	}
	must(os.WriteFile(dstPrompt, promptOut.Bytes(), 0644))
	log.Println("Generated:", dstPrompt)

}

// scriptDir returns the directory containing the currently executing Go script.
func scriptDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("Could not get caller information")
	}
	return filepath.Dir(file)
}

// must is a helper that panics if the error is non-nil.
func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// Note: The Go template string has been moved to defs_gen.tmpl
// Note: The system prompt template is now in spec.tmpl
