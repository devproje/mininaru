// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

type local struct {
	Name string
	Type string
	Err  bool
}

type rewriter struct {
	info        *types.Info
	packagePath string
	names       map[types.Object]string
	locals      []local
	used        map[string]string
	byBase      map[string]string
	changed     bool
}

func fieldNames(fields *ast.FieldList, used map[string]string) {
	var field *ast.Field
	var name *ast.Ident

	if fields == nil {
		return
	}

	for _, field = range fields.List {
		for _, name = range field.Names {
			used[name.Name] = "parameter"
		}
	}
}

func typeName(value types.Type, packagePath string) string {
	return types.TypeString(value, func(pkg *types.Package) string {
		if pkg == nil {
			return ""
		}
		if pkg.Path() == packagePath {
			return ""
		}

		return pkg.Name()
	})
}

func uniqueName(base, kind string, used map[string]string) string {
	var candidate string
	var index int
	var found bool

	_, found = used[base]
	if !found {
		return base
	}

	candidate = base + "Value"
	for index = 2; ; index++ {
		_, found = used[candidate]
		if !found {
			return candidate
		}
		candidate = fmt.Sprintf("%sValue%d", base, index)
	}
}

func (r *rewriter) add(identifier *ast.Ident) {
	var object types.Object
	var base string
	var rendered string
	var name string
	var existing string
	var found bool

	if identifier == nil || identifier.Name == "_" {
		return
	}

	object = r.info.Defs[identifier]
	if object == nil {
		return
	}
	if _, found = r.names[object]; found {
		return
	}

	base = identifier.Name
	rendered = typeName(object.Type(), r.packagePath)
	if strings.Contains(rendered, "impl.messageState") {
		rendered = "messageState"
	}
	existing, found = r.byBase[base]
	if found && r.used[existing] == rendered {
		r.names[object] = existing
		return
	}

	name = uniqueName(base, rendered, r.used)
	r.names[object] = name
	r.used[name] = rendered
	if !found {
		r.byBase[base] = name
	}
	r.locals = append(r.locals, local{Name: name, Type: rendered, Err: base == "err"})
}

func (r *rewriter) collect(node ast.Node) bool {
	var assignment *ast.AssignStmt
	var rangeStatement *ast.RangeStmt
	var expression ast.Expr
	var identifier *ast.Ident
	var nested bool

	if node == nil {
		return false
	}

	_, nested = node.(*ast.FuncLit)
	if nested {
		return false
	}

	assignment, _ = node.(*ast.AssignStmt)
	if assignment != nil && assignment.Tok == token.DEFINE {
		for _, expression = range assignment.Lhs {
			identifier, _ = expression.(*ast.Ident)
			r.add(identifier)
		}
		assignment.Tok = token.ASSIGN
		r.changed = true
	}

	rangeStatement, _ = node.(*ast.RangeStmt)
	if rangeStatement != nil && rangeStatement.Tok == token.DEFINE {
		identifier, _ = rangeStatement.Key.(*ast.Ident)
		r.add(identifier)
		identifier, _ = rangeStatement.Value.(*ast.Ident)
		r.add(identifier)
		rangeStatement.Tok = token.ASSIGN
		r.changed = true
	}

	return true
}

func (r *rewriter) rename(node ast.Node) bool {
	var identifier *ast.Ident
	var object types.Object
	var name string
	var found bool

	if node == nil {
		return false
	}

	identifier, _ = node.(*ast.Ident)
	if identifier == nil {
		return true
	}

	object = r.info.ObjectOf(identifier)
	name, found = r.names[object]
	if found {
		identifier.Name = name
	}

	return true
}

func localSpecs(locals []local) []ast.Spec {
	var specs []ast.Spec
	var current local
	var expression ast.Expr

	var err error

	sort.SliceStable(locals, func(left, right int) bool {
		return !locals[left].Err && locals[right].Err
	})

	for _, current = range locals {
		expression, err = parser.ParseExpr(current.Type)
		if err != nil {
			panic(err)
		}
		specs = append(specs, &ast.ValueSpec{Names: []*ast.Ident{ast.NewIdent(current.Name)}, Type: expression})
	}

	return specs
}

func normalizeTypeBuilder(body *ast.BlockStmt) {
	var index int
	var first *ast.AssignStmt
	var second *ast.AssignStmt
	var identifier *ast.Ident
	var selector *ast.SelectorExpr
	var source *ast.Ident

	for index = 0; index+1 < len(body.List); index++ {
		first, _ = body.List[index].(*ast.AssignStmt)
		second, _ = body.List[index+1].(*ast.AssignStmt)
		if first == nil || second == nil || first.Tok != token.DEFINE || len(first.Lhs) != 1 || len(first.Rhs) != 1 || len(second.Lhs) != 1 || len(second.Rhs) != 1 {
			continue
		}

		identifier, _ = first.Lhs[0].(*ast.Ident)
		selector, _ = second.Rhs[0].(*ast.SelectorExpr)
		if identifier == nil || identifier.Name != "out" || selector == nil || selector.Sel.Name != "File" {
			continue
		}
		source, _ = selector.X.(*ast.Ident)
		if source == nil || source.Name != identifier.Name {
			continue
		}

		first.Lhs = second.Lhs
		first.Rhs[0] = &ast.SelectorExpr{X: first.Rhs[0], Sel: ast.NewIdent("File")}
		body.List = append(body.List[:index+1], body.List[index+2:]...)
		return
	}
}

func rewriteBody(receiver *ast.FieldList, function *ast.FuncType, body *ast.BlockStmt, info *types.Info, packagePath string) {
	var current rewriter
	var declaration *ast.DeclStmt

	if body == nil {
		return
	}

	current = rewriter{info: info, packagePath: packagePath, names: make(map[types.Object]string), used: make(map[string]string), byBase: make(map[string]string)}
	fieldNames(receiver, current.used)
	fieldNames(function.Params, current.used)
	fieldNames(function.Results, current.used)
	normalizeTypeBuilder(body)
	ast.Inspect(body, current.collect)
	if !current.changed {
		return
	}

	ast.Inspect(body, current.rename)
	declaration = &ast.DeclStmt{Decl: &ast.GenDecl{Tok: token.VAR, Specs: localSpecs(current.locals)}}
	body.List = append([]ast.Stmt{declaration}, body.List...)
}

func rewriteFile(path string, file *ast.File, info *types.Info, files *token.FileSet, packagePath string) error {
	var declaration ast.Decl
	var function *ast.FuncDecl
	var output *os.File

	var err error

	for _, declaration = range file.Decls {
		function, _ = declaration.(*ast.FuncDecl)
		if function == nil || function.Body == nil {
			continue
		}

		rewriteBody(function.Recv, function.Type, function.Body, info, packagePath)
	}

	output, err = os.Create(path)
	if err != nil {
		return err
	}
	defer output.Close()

	return format.Node(output, files, file)
}

func run() error {
	var config packages.Config
	var loaded []*packages.Package
	var pkg *packages.Package
	var index int
	var path string

	var err error

	config.Mode = packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax |
		packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps
	loaded, err = packages.Load(&config, "./rpc/gen/mininaru/v1")
	if err != nil {
		return err
	}
	if packages.PrintErrors(loaded) != 0 || len(loaded) != 1 {
		return fmt.Errorf("load generated protobuf package")
	}

	pkg = loaded[0]
	for index, path = range pkg.CompiledGoFiles {
		if !strings.HasSuffix(path, ".pb.go") || !strings.HasPrefix(path, filepath.Clean("rpc")+string(filepath.Separator)) && !filepath.IsAbs(path) {
			continue
		}
		err = rewriteFile(path, pkg.Syntax[index], pkg.TypesInfo, pkg.Fset, pkg.Types.Path())
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	var err error

	err = run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
