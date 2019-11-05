// hcltemplate is a filter program for rendering JSON input to textual output
// using the HCL template language.
package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	flag "github.com/spf13/pflag"
)

var GitCommit string
var Version = "v0.0.0"
var Prerelease = "dev"

func main() {
	flag.Usage = usage

	versionP := flag.BoolP("version", "v", false, "show version information")
	outP := flag.StringP("out", "o", "-", "output to a given file, instead of stdout")
	flag.Parse()

	if *versionP {
		versionStr := Version
		if Prerelease != "" {
			versionStr = versionStr + "-" + Prerelease
		}
		fmt.Printf("terraform-filter-vars %s\n", versionStr)
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	var diags []tfconfig.Diagnostic

	modDir := args[0]
	mod, moreDiags := tfconfig.LoadModule(modDir)
	diags = append(diags, moreDiags...)
	exitIfErrors(diags)

	wantedVars := make([]string, 0, len(mod.Variables))
	wantedVarsSet := make(map[string]struct{}, len(mod.Variables))
	for name := range mod.Variables {
		wantedVars = append(wantedVars, name)
		wantedVarsSet[name] = struct{}{}
	}
	sort.Strings(wantedVars)

	attrs := make(map[string]*hclwrite.Attribute, len(wantedVars))
	varFilePaths := args[1:]
	for _, varFilePath := range varFilePaths {
		if strings.HasSuffix(varFilePath, ".json") {
			// For now we don't support JSON, because our output is a single
			// native syntax vars definition. With some care we could
			// potentially transform JSON expressions into native syntax ones,
			// but that's tricky to get right and so we'll just stick to the
			// common case of native syntax input files for now.
			diags = append(diags, tfconfig.Diagnostic{
				Severity: tfconfig.DiagError,
				Summary:  "JSON tfvars not supported",
				Detail:   fmt.Sprintf("Can't read %s: only native syntax .tfvars files are supported.", varFilePath),
			})
			continue
		}

		varFileSrc, err := ioutil.ReadFile(varFilePath)
		if err != nil {
			diags = append(diags, tfconfig.Diagnostic{
				Severity: tfconfig.DiagError,
				Summary:  "Failed to read input file",
				Detail:   fmt.Sprintf("Can't read %s: %s.", varFilePath, err),
			})
			continue
		}

		varFile, hclDiags := hclwrite.ParseConfig(varFileSrc, varFilePath, hcl.Pos{Line: 1, Column: 1})
		diags = appendHCLDiags(diags, hclDiags)
		if hclDiags.HasErrors() {
			continue
		}

		for name, attr := range varFile.Body().Attributes() {
			if _, exists := wantedVarsSet[name]; !exists {
				continue // ignore undeclared
			}
			// If multiple files define the same variable, we'll override
			// previous definitions here so that the last one in the sequence
			// "wins", which is consistent with Terraform's own interpretation
			// of multiple -var-file arguments.
			attrs[name] = attr
		}
	}
	exitIfErrors(diags)

	outF := hclwrite.NewEmptyFile()
	outBody := outF.Body()
	for _, name := range wantedVars {
		attr, ok := attrs[name]
		if !ok {
			continue
		}

		// We're not going to do any further wrangling of the attributes, so
		// for simplicity we'll just paste them in as unstructured tokens
		// to our output file. That avoids book-keeping around detaching and
		// re-attaching, because the sequence of tokens will be reconstructed
		// here.
		outBody.AppendUnstructuredTokens(attr.BuildTokens(nil))
	}

	var outWr *os.File
	switch *outP {
	case "-":
		outWr = os.Stdout
	default:
		var err error
		outWr, err = os.Create(*outP)
		if err != nil {
			diags = append(diags, tfconfig.Diagnostic{
				Severity: tfconfig.DiagError,
				Summary:  "Failed to open output file",
				Detail:   fmt.Sprintf("Can't create %s: %s.", *outP, err),
			})
			exitWithDiags(diags)
		}
	}

	_, err := outF.WriteTo(outWr)
	if err != nil {
		diags = append(diags, tfconfig.Diagnostic{
			Severity: tfconfig.DiagError,
			Summary:  "Failed to write to output file",
			Detail:   fmt.Sprintf("Error writing to %s: %s.", *outP, err),
		})
		exitWithDiags(diags)
	}

	exitWithDiags(diags)
}

func showDiags(diags []tfconfig.Diagnostic) {
	for _, diag := range diags {
		var prefixStr string
		switch diag.Severity {
		case tfconfig.DiagError:
			prefixStr = "Error: "
		case tfconfig.DiagWarning:
			prefixStr = "Warning: "
		}

		if diag.Pos != nil {
			prefixStr = fmt.Sprintf("%s (%s:%d) ", prefixStr, diag.Pos.Filename, diag.Pos.Line)
		}

		fmt.Fprintf(os.Stderr, "%s%s; %s", prefixStr, diag.Summary, diag.Detail)
	}
}

func exitWithDiags(diags []tfconfig.Diagnostic) {
	showDiags(diags)
	for _, diag := range diags {
		if diag.Severity == tfconfig.DiagError {
			os.Exit(1)
		}
	}
	os.Exit(0)
}

func exitIfErrors(diags []tfconfig.Diagnostic) {
	for _, diag := range diags {
		if diag.Severity == tfconfig.DiagError {
			showDiags(diags)
			os.Exit(1)
		}
	}
}

func appendHCLDiags(diags []tfconfig.Diagnostic, hclDiags hcl.Diagnostics) []tfconfig.Diagnostic {
	for _, hclDiag := range hclDiags {
		var severity tfconfig.DiagSeverity
		switch hclDiag.Severity {
		case hcl.DiagError:
			severity = tfconfig.DiagError
		case hcl.DiagWarning:
			severity = tfconfig.DiagWarning
		}
		var pos *tfconfig.SourcePos
		if hclDiag.Subject != nil {
			pos = &tfconfig.SourcePos{
				Filename: hclDiag.Subject.Filename,
				Line:     hclDiag.Subject.Start.Line,
			}
		}

		diags = append(diags, tfconfig.Diagnostic{
			Severity: severity,
			Summary:  hclDiag.Summary,
			Detail:   hclDiag.Detail,
			Pos:      pos,
		})
	}

	return diags
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: terraform-filter-vars <module-dir> [tfvars-files...]\n\nReads the given tfvars files and produces output in tfvars format containing only definitions for variables declared in the given module.\n\n")
}
