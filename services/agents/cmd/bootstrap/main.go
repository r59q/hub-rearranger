// Command bootstrap prepares an AW-008 package locally; it never writes GitHub.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
	"github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/bootstrap"
	"github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/profiles"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(arguments []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
	flags.SetOutput(stderr)
	source := flags.String("source", ".", "reviewed Hub Rearranger checkout containing canonical artifacts")
	target := flags.String("target", "", "existing target checkout to compare (omit for an empty repository)")
	output := flags.String("output", "", "new staging directory for the complete proposed package")
	format := flags.String("format", "json", "review format: json or diff (requires Git)")
	if flags.Parse(arguments) != nil || flags.NArg() != 0 || (*format != "json" && *format != "diff") {
		fmt.Fprintln(stderr, "Use --source, optional --target/--output, and --format json|diff.")
		return 1
	}
	if *output != "" && !bootstrap.SeparateOutput(*output, *source, *target) {
		fmt.Fprintln(stderr, "Output must have an existing parent and be outside the source and target checkouts.")
		return 1
	}
	sourceRoot, err := os.OpenRoot(*source)
	if err != nil {
		fmt.Fprintln(stderr, "Cannot open the reviewed source checkout.")
		return 1
	}
	defer sourceRoot.Close()
	templates, err := bootstrap.Templates(sourceRoot)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	existing := map[string]string{}
	if *target != "" {
		targetRoot, err := os.OpenRoot(*target)
		if err != nil {
			fmt.Fprintln(stderr, "Cannot open the target checkout.")
			return 1
		}
		defer targetRoot.Close()
		existing, err = bootstrap.Snapshot(targetRoot, templates)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	validator, err := profiles.NewValidator()
	if err != nil {
		fmt.Fprintln(stderr, "Canonical profile validator unavailable.")
		return 1
	}
	plan := domain.PlanBootstrap(templates, existing, validator)
	if err := render(stdout, *format, plan, existing); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(plan.Diagnostics) > 0 {
		for _, diagnostic := range plan.Diagnostics {
			fmt.Fprintf(stderr, "%s: %s: %s\n", diagnostic.Path, diagnostic.Code, diagnostic.Message)
		}
		return 2
	}
	if *output != "" {
		if err := bootstrap.WritePackage(*output, plan); err != nil {
			fmt.Fprintln(stderr, "Cannot create the package; output must be a new directory with an existing parent.")
			return 1
		}
	}
	return 0
}

func render(output io.Writer, format string, plan domain.BootstrapPlan, existing map[string]string) error {
	if format == "json" {
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(plan)
	}
	diff, err := bootstrap.Diff(context.Background(), plan, existing)
	if err != nil {
		return err
	}
	_, err = output.Write(diff)
	return err
}
