package code

import (
	_ "embed"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

//go:embed tmpl
var interfaceTemplate string

type callbackFileFilter func(string) bool
type StructName string

// InterfaceDefinition defines the parameters for the interface template.
type InterfaceDefinition struct {
	PackageName string
	AllImports  []string
	Structs     map[StructName][]FuncSign
}

type StructFile struct {
	Header []string
	Body   map[StructName][]FuncSign
}

type FuncSign struct {
	Name    string
	Params  string
	Results string
	Comment string
}

var tmpl *template.Template

func init() {
	var err error
	tmpl, err = template.New("genGo").Funcs(template.FuncMap{
		"importsHelper": func(imports []string) string {
			importBody := "\nimport "
			switch len(imports) {
			case 0:
				return ""
			case 1:
				return importBody + imports[0]
			default:
				importBody += "(\n\t" + strings.Join(imports, "\n\t") + "\n)"
			}
			return importBody
		},
		"commentHelper": func(comment string) string {
			if len(comment) > 0 {
				return comment + "\n\t"
			}
			return comment
		},
		"resultHelper": func(res string) string {
			if len(strings.Split(res, " ")) > 1 {
				return "(" + res + ")"
			}
			return res
		},
	}).Parse(interfaceTemplate)
	if err != nil {
		panic(fmt.Sprintf("load template err:%v", err))
	}
}

func MakeGenFile(rootDir string, opts ...callbackFileFilter) (err error) {
	fset := token.NewFileSet()
	mapDirPath := make(map[string]map[string][]StructFile)
	err = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(filepath.Base(path), "_interface.go") {
			return nil
		}
		if strings.HasPrefix(filepath.Base(path), "mock_") {
			return nil
		}
		if !strings.HasSuffix(filepath.Base(path), ".go") {
			return nil
		}
		for _, callback := range opts {
			if callback(path) {
				return nil
			}
		}
		pkgName, structFile := astFile(fset, path)
		if _, ok := mapDirPath[filepath.Dir(path)]; ok {
			mapDirPath[filepath.Dir(path)][pkgName] = append(mapDirPath[filepath.Dir(path)][pkgName], structFile)
		} else {
			mapDirPath[filepath.Dir(path)] = map[string][]StructFile{
				pkgName: {structFile},
			}
		}
		return nil
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	genFile(mapDirPath)
	return
}

func parseFuncDel(f *ast.FuncDecl) (structName, methodName string, params, result []string) {
	methodName = f.Name.Name
	structName = f.Recv.List[0].Type.(*ast.StarExpr).X.(*ast.Ident).Name
	for _, p := range f.Type.Params.List {
		name := p.Names[0].Name + " "
		t := p.Type.(*ast.Ident).Name
		params = append(params, name+t)
	}
	if f.Type.Results != nil {
		for _, res := range f.Type.Results.List {
			t := res.Type.(*ast.Ident).Name
			if len(res.Names) > 0 {
				t = res.Names[0].Name + " " + t
			}
			result = append(result, t)
		}
	}
	return
}

func astFile(fset *token.FileSet, file string) (pkgName string, structsFile StructFile) {
	astFile, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		panic(fmt.Sprintf("astFile parse err:%v", err))
	}
	pkgName = astFile.Name.Name
	body := make(map[StructName][]FuncSign, 0)
	for _, i := range astFile.Imports {
		if i.Name != nil {
			structsFile.Header = append(structsFile.Header, fmt.Sprintf("%s %s", i.Name.String(), i.Path.Value))
		} else {
			structsFile.Header = append(structsFile.Header, i.Path.Value)
		}
	}

	for _, decl := range astFile.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if funcDecl.Recv == nil {
			continue
		}
		if !funcDecl.Name.IsExported() {
			continue
		}
		comm := []string{}
		if funcDecl.Doc != nil {
			for _, doc := range funcDecl.Doc.List {
				comm = append(comm, doc.Text)
			}
		}
		sName, mName, params, ret := parseFuncDel(funcDecl)
		body[StructName(sName)] = append(body[StructName(sName)], FuncSign{
			Name:    mName,
			Params:  strings.Join(params, ", "),
			Results: strings.Join(ret, ", "),
			Comment: strings.Join(comm, "\n\t"),
		})
	}
	structsFile.Body = body
	return
}

func genFile(makeDir map[string]map[string][]StructFile) error {
	for dir, pkg := range makeDir {
		for pkgName, structFiles := range pkg {
			fileName := dir + "/" + pkgName + "_interface.go"
			out, err := os.Create(fileName)
			if err != nil {
				panic(fmt.Sprintf("genFile fail:%v", err))
			}
			allImport := []string{}
			structMap := make(map[StructName][]FuncSign)
			for _, file := range structFiles {
				allImport = append(allImport, file.Header...)
				for structName, funcs := range file.Body {
					structMap[structName] = append(structMap[structName], funcs...)
				}
			}
			allImport = uniq(allImport)
			ifacedef := &InterfaceDefinition{
				PackageName: pkgName,
				AllImports:  allImport,
				Structs:     structMap,
			}
			err = tmpl.Execute(out, ifacedef)
			if err != nil {
				panic(fmt.Sprintf("gen file fail:%v", err))
			}
			out.Close()
			fmt.Printf("[gen file] %s %s\n", time.Now().Format(time.DateTime), fileName)
		}
	}
	return nil
}

func uniq(ss []string) (uniqs []string) {
	exist := map[string]struct{}{}
	for _, s := range ss {
		if _, ok := exist[s]; !ok {
			uniqs = append(uniqs, s)
			exist[s] = struct{}{}
		}
	}
	return
}
