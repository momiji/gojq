package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"io"
	"os"
	"regexp"

	"github.com/itchyny/astgen-go"
	"github.com/itchyny/gojq"
)

const fileFormat = `// Code generated by _tools/gen_builtin.go; DO NOT EDIT.

package gojq

func init() {%s}
`

func main() {
	var output string
	flag.StringVar(&output, "o", "", "output file")
	flag.Parse()
	if err := run(output); err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
}

func run(output string) error {
	qs := make(map[string]*gojq.Query)
	for n, src := range gojq.BuiltinFuncDefinitions {
		q, err := gojq.Parse(src)
		if err != nil {
			return err
		}
		qs[n] = q
	}
	t, err := astgen.Build(qs)
	if err != nil {
		return err
	}
	f, ok := t.(*ast.CallExpr)
	if !ok {
		return &unexpectedAstError{t}
	}
	var buf bytes.Buffer
	if err := printAssignStmtsFromFuncArgs(&buf, f); err != nil {
		return err
	}
	buf.Write([]byte("\n\tbuiltinFuncs = "))
	m, err := getReturnStmtCompositeLit(f)
	if err != nil {
		return err
	}
	if err := printCompositeLit(&buf, m); err != nil {
		return err
	}
	out := os.Stdout
	if output != "" {
		f, err := os.Create(output)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}
	_, err = fmt.Fprintf(out, fileFormat, buf.String())
	return err
}

func printAssignStmtsFromFuncArgs(out io.Writer, f *ast.CallExpr) error {
	fn, ok := f.Fun.(*ast.ParenExpr).X.(*ast.FuncLit)
	if !ok {
		return &unexpectedAstError{f}
	}
	lhs := []ast.Expr{}
	rhs := []ast.Expr{}
	var i int
	for _, param := range fn.Type.Params.List {
		for _, name := range param.Names {
			lhs = append(lhs, name)
			rhs = append(rhs, f.Args[i])
			i++
		}
	}
	stmt := &ast.AssignStmt{Tok: token.DEFINE, Lhs: lhs, Rhs: rhs}
	out.Write([]byte("\n\t"))
	if err := printer.Fprint(out, token.NewFileSet(), stmt); err != nil {
		return err
	}
	return nil
}

func getReturnStmtCompositeLit(f *ast.CallExpr) (*ast.CompositeLit, error) {
	fn, ok := f.Fun.(*ast.ParenExpr).X.(*ast.FuncLit)
	if !ok {
		return nil, &unexpectedAstError{f}
	}
	if len(fn.Body.List) != 1 {
		return nil, &unexpectedAstError{fn.Body}
	}
	r, ok := fn.Body.List[0].(*ast.ReturnStmt)
	if !ok {
		return nil, &unexpectedAstError{fn.Body.List[0]}
	}
	if len(r.Results) != 1 {
		return nil, &unexpectedAstError{r}
	}
	m, ok := r.Results[0].(*ast.CompositeLit)
	if !ok {
		return nil, &unexpectedAstError{r.Results[0]}
	}
	return m, nil
}

func printCompositeLit(out io.Writer, t *ast.CompositeLit) error {
	err := printer.Fprint(out, token.NewFileSet(), t.Type)
	if err != nil {
		return err
	}
	out.Write([]byte("{"))
	for _, kv := range t.Elts {
		out.Write([]byte("\n\t\t"))
		var kvBuf bytes.Buffer
		err = printer.Fprint(&kvBuf, token.NewFileSet(), kv)
		if err != nil {
			return err
		}
		str := kvBuf.String()
		for op := gojq.OpAdd; op <= gojq.OpAlt; op++ {
			r := regexp.MustCompile(fmt.Sprintf(`\bOp: %d\b`, op))
			str = r.ReplaceAllString(str, fmt.Sprintf("Op: %#v", op))
		}
		out.Write([]byte(str))
		out.Write([]byte(","))
	}
	out.Write([]byte("\n\t}\n"))
	return nil
}

type unexpectedAstError struct{ X ast.Node }

func (err *unexpectedAstError) Error() string {
	return fmt.Sprintf("unexpected ast: %#v", err.X)
}