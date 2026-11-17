// nibgen generates TypeScript bindings, with an optional nib.cli/v1 machine result.
package main

import (
	"flag"
	"fmt"
	"io"
	"nib.dev/nib/binding"
	"nib.dev/nib/cli"
	"os"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
func run(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("nibgen", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonMode := fs.Bool("json", false, "emit nib.cli/v1 JSON")
	output := fs.String("out", "./bindings", "TS output directory")
	help := fs.Bool("help", false, "show help")
	fs.BoolVar(help, "h", false, "show help")
	r := &cli.Reporter{JSON: cli.WantsJSON(fs, args), Command: "nibgen", Out: out, Err: errOut}
	if err := cli.Parse(fs, args); err != nil {
		return r.Fail("cli/invalid_flag", err.Error(), "Use nibgen --help for flags.", cli.ExitUsage)
	}
	r.JSON = *jsonMode
	if *help {
		if !r.JSON {
			fmt.Fprintln(out, "usage: nibgen [--json] [-out dir] [pkgdir]")
		}
		return r.Finish(map[string]any{"usage": "nibgen [--json] [-out dir] [pkgdir]"})
	}
	if fs.NArg() > 1 {
		return r.Fail("cli/invalid_args", "at most one package directory is allowed", "Use nibgen -out <directory> <pkgdir>.", cli.ExitUsage)
	}
	dir := fs.Arg(0)
	if dir == "" {
		dir = "."
	}
	pkg, err := binding.Load(dir)
	if err != nil {
		return r.Fail("nibgen/load", err.Error(), "Use one valid Go package directory with supported service signatures.", cli.ExitFailure)
	}
	if len(pkg.Services) == 0 {
		return r.Fail("nibgen/no_services", "no binding services in "+dir, "Export a struct with exported methods returning (T, error).", cli.ExitFailure)
	}
	rep, err := binding.Generate(pkg, *output)
	if err != nil {
		return r.Fail("nibgen/generate", err.Error(), "Check output directory permissions and existing generated files.", cli.ExitFailure)
	}
	if !r.JSON {
		fmt.Fprintf(out, "nibgen: %d 个服务 -> %s\n", len(pkg.Services), *output)
		for _, f := range rep.Written {
			fmt.Fprintln(out, "  write  "+f)
		}
		for _, f := range rep.Unchanged {
			fmt.Fprintln(out, "  same   "+f)
		}
		for _, f := range rep.Removed {
			fmt.Fprintln(out, "  remove "+f)
		}
	}
	return r.Finish(map[string]any{"services": len(pkg.Services), "output": *output, "written": rep.Written, "unchanged": rep.Unchanged, "removed": rep.Removed})
}
