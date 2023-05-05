package main

import (
	"bytes"
	"fmt"
	"github.com/dave/dst/decorator"
	"github.com/misnaged/annales/logger"
	"github.com/misnaged/import_organizer/package_collector"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"golang.org/x/tools/go/ast/astutil"
	"log"
	"os"
	"strconv"
	"strings"

	_ "unsafe"

	_ "go/format"
)

//go:linkname parse go/format.parse
func parse(fset *token.FileSet, filename string, src []byte, fragmentOk bool) (
	file *ast.File,
	sourceAdj func(src []byte, indent int) []byte,
	indentAdj int,
	err error,
)

//go:linkname format go/format.format
func format(
	fset *token.FileSet,
	file *ast.File,
	sourceAdj func(src []byte, indent int) []byte,
	indentAdj int,
	src []byte,
	cfg printer.Config,
) ([]byte, error)

type FileStruct struct {
	pkg, imports, body []byte
}

type AbstractSxTree struct {
	byteMap  map[string][]byte
	advanced map[string]string
	//advM            map[string][]string
	fset            *token.FileSet
	astFile         *ast.File
	decl            []ast.Decl
	formattedOutput []byte
	sourceAdj       func(src []byte, indent int) []byte
	indentAdj       int
	stop            chan struct{}
	ready           chan bool
}

func (abs *AbstractSxTree) VanillaCleanerOld(b []byte, file string) error {
	err := abs.NewTree(b, file)
	if err != nil {
		return fmt.Errorf("NewTree in ClearImports %w", err)
	}
	abs.astFile, err = parser.ParseFile(abs.fset, file, b, parser.ParseComments)
	if err != nil {
		log.Fatalln(err)
	}
	astutil.DeleteImport(abs.fset, abs.astFile, file)

	for i := range abs.astFile.Imports {
		abs.astFile.Imports[i].Path.Value = ""
		abs.astFile.Imports[i].Path.ValuePos = 0
		abs.astFile.Imports[i].Comment = nil
		abs.astFile.Imports[i].Doc = nil
		abs.astFile.Imports[i].Name = nil

	}

	out, err := format(abs.fset, abs.astFile, abs.sourceAdj, abs.indentAdj, b, cfg)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	trimmed, err := abs.TrimSpace(file, out)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	err = os.WriteFile(file, trimmed, os.ModeAppend)
	if err != nil {
		return fmt.Errorf(" WriteFile %w", err)
	}
	return nil
}

func (abs *AbstractSxTree) VanillaCleaner(files []string) error {
	for i := range files {
		if ImportCheck(files[i]) {
			b, err := os.ReadFile(files[i])
			if err != nil {
				return fmt.Errorf("error ReadFile %w", err)
			}
			if err = abs.NewTree(b, files[i]); err != nil {
				return fmt.Errorf("NewTree in VanillaCleaner %w", err)
			}
			abs.astFile, err = parser.ParseFile(abs.fset, files[i], b, parser.ParseComments)
			if err != nil {
				log.Fatalln(err)
			}
			astutil.DeleteImport(abs.fset, abs.astFile, files[i])

			for ii := range abs.astFile.Imports {
				abs.astFile.Imports[ii].Path.Value = ""
				abs.astFile.Imports[ii].Path.ValuePos = 0
				abs.astFile.Imports[ii].Comment = nil
				abs.astFile.Imports[ii].Doc = nil
				abs.astFile.Imports[ii].Name = nil

			}

			out, err := format(abs.fset, abs.astFile, abs.sourceAdj, abs.indentAdj, b, cfg)
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			trimmed, err := abs.TrimSpace(files[i], out)
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			err = os.WriteFile(files[i], trimmed, os.ModeAppend)
			if err != nil {
				return fmt.Errorf(" WriteFile %w", err)
			}
		}
	}
	return nil
}

func (abs *AbstractSxTree) DaveCleaner(files []string) error {

	for i := range files {
		if ImportCheck(files[i]) {
			b, err := os.ReadFile(files[i])
			if err != nil {
				return fmt.Errorf("error ReadFile %w", err)
			}
			if err = abs.NewTree(b, files[i]); err != nil {
				return fmt.Errorf("NewTree in DaveCleaner %w", err)
			}

			f, err := decorator.Parse(b)
			if err != nil {
				return fmt.Errorf("decorator.Parse failed: %w", err)
			}

			for ii := range f.Imports {
				f.Imports[ii].Decorations().Start.Replace("")
				f.Imports[ii].Decorations().End.Replace("")
				f.Imports[ii].Decs.Start.Replace("")
				f.Imports[ii].Decs.End.Replace("")

				f.Imports[ii].Decs.NodeDecs.Start.Replace("")
				f.Imports[ii].Decs.NodeDecs.End.Replace("")
				f.Imports[ii].Decorations().Start.Replace("")
			}

			err = os.WriteFile(files[i], []byte(""), 0666)
			if err != nil {
				return fmt.Errorf(" WriteFile %w", err)
			}
			fileToOpen, err := os.OpenFile(files[i], os.O_APPEND|os.O_WRONLY, os.ModeAppend)
			if err != nil {
				return fmt.Errorf(" failed to open file: %w", err)
			}
			err = decorator.Fprint(fileToOpen, f)
			if err != nil {
				return fmt.Errorf(" printing decoration to the file failed: %w", err)
			}
		}
	}

	return nil
}

func (abs *AbstractSxTree) TrimSpace(file string, b []byte) ([]byte, error) {
	if err := abs.NewTree(b, file); err != nil {
		return nil, fmt.Errorf("NewTree in TrimSpace %w", err)
	}

	for idx := 0; idx < len(abs.astFile.Decls); idx++ {
		d := abs.astFile.Decls[idx]
		switch d.(type) {
		case *ast.FuncDecl:
		case *ast.GenDecl:
			dd := d.(*ast.GenDecl)
			if dd.Tok == token.IMPORT {
				iSpec := &ast.ImportSpec{Path: &ast.BasicLit{Value: strings.TrimSpace("")}}
				dd.Specs = append(dd.Specs, iSpec)
			}
		}
	}
	out, err := format(abs.fset, abs.astFile, abs.sourceAdj, abs.indentAdj, b, cfg)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return out, nil
}

func (abs *AbstractSxTree) NewTree(b []byte, filename string) error {
	abs.fset = token.NewFileSet()
	file, sourceAdj, ident, err := parse(abs.fset, filename, b, true)
	if err != nil {
		return fmt.Errorf("NewTree %w", err)
	}
	abs.astFile, abs.sourceAdj, abs.indentAdj = file, sourceAdj, ident
	return nil
}

func (abs *AbstractSxTree) readImports(str []string, file string, b []byte) error {
	err := abs.NewTree(b, file)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	for idx := 0; idx < len(abs.astFile.Decls); idx++ {
		d := abs.astFile.Decls[idx]
		switch d.(type) {
		case *ast.FuncDecl:
		case *ast.GenDecl:

			dd := d.(*ast.GenDecl)
			if dd.Tok == token.IMPORT {
				for ii := range str {
					if str[ii] == "\n" {
						iSpec := &ast.ImportSpec{Path: &ast.BasicLit{Value: str[ii]}}
						dd.Specs = append(dd.Specs, iSpec)
					}
				}
			}
		}

	}

	_, err = format(abs.fset, abs.astFile, abs.sourceAdj, abs.indentAdj, b, cfg)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

var (
	kek, _  = os.Create("kek.txt")
	lol     = log.New(kek, "", 0)
	kek2, _ = os.Create("kek2.txt")
	lol2    = log.New(kek2, "", 0)
)

func (abs *AbstractSxTree) ClearImports(files []string) error {
	err := abs.DaveCleaner(files)
	if err != nil {
		return fmt.Errorf("DaveCleaner import %w", err)
	}

	err = abs.VanillaCleaner(files)
	if err != nil {
		return fmt.Errorf("vanilla clear import %w", err)
	}
	return nil
}

//func (abs *AbstractSxTree) Read(files []string) error {
//	for i := range files {
//		if ImportCheck(files[i]) {
//			str := abs.MergeImportsSlices(files[i])
//
//			b, err := os.ReadFile(files[i])
//			if err != nil {
//				logger.Log().Errorf("error ReadFile %v", err)
//				os.Exit(1)
//			}
//			 imports := abs.ImportsFromFiles(files[i], b)
//			 //abs.advM[files[i]] = str
//			 k, _ := abs.GetKeys()
//			 err = abs.readImports(str, files[i], b)
//			 if err != nil {
//			 	logger.Log().Errorf("readImports %v", err)
//			 	os.Exit(1)
//			 }
//		}
//	}
//	return nil
//}

func (abs *AbstractSxTree) WriteImps(files []string) {
	for i := range files {
		if ImportCheck(files[i]) {
			str := abs.MergeImportsSlices(files[i])
			r, err := os.ReadFile(files[i])
			if err != nil {
				logger.Log().Errorf("error ReadFile %v", err)
				os.Exit(1)
			}
			imports := abs.ImportsFromFiles(r, files[i])

			abs.CombineIntoPath(imports)

			err = abs.VanillaCleanerOld(r, files[i])
			if err != nil {
				logger.Log().Errorf("vanilla clear import %v", err)
				os.Exit(1)
			}
			//err := abs.DaveCleaner(r, files[i])
			//if err != nil {
			//	logger.Log().Errorf("dave clear import %v", err)
			//	os.Exit(1)
			//}
			re, err := os.ReadFile(files[i])
			if err != nil {
				logger.Log().Errorf("error ReadFile %v", err)
				os.Exit(1)
			}

			//if err = abs.TrimSpace(re, files[i]); err != nil {
			//	logger.Log().Errorf("TrimSpace %v", err)
			//	os.Exit(1)
			//}
			str = abs.Sort(str)

			if err = abs.WriteImports(re, str, files[i]); err != nil {
				logger.Log().Errorf("WriteImports %v", err)
				os.Exit(1)
			}
		}
	}

}
func main() {
	package_collector.Populate()
	package_collector.Std()
	package_collector.ExternalPackages()
	//lol3.Println(package_collector.Packages)
	files := package_collector.GoFiles()
	abs := &AbstractSxTree{
		byteMap:  make(map[string][]byte),
		advanced: make(map[string]string),
		//advM:     make(map[string][]string),
	}

	//if err := abs.Read(files); err != nil {
	//	logger.Log().Errorf("error ReadFile %v", err)
	//	os.Exit(1)
	//}
	//
	abs.WriteImps(files)

	err := abs.ClearImports(files)
	if err != nil {
		logger.Log().Errorf("error ReadFile %v", err)
		os.Exit(1)
	}

}

func (abs *AbstractSxTree) ImportsFromFiles(b []byte, file string) []*ast.ImportSpec {
	err := abs.NewTree(b, file)
	if err != nil {
		logger.Log().Error(err.Error())
		return nil
	}
	return abs.astFile.Imports
}

func (abs *AbstractSxTree) GetKeys() ([]string, []string) {
	var keys, values []string
	for k, v := range abs.advanced {
		k, _ = strconv.Unquote(k)
		keys = append(keys, k)
		values = append(values, v)
	}

	return keys, values
}
func (abs *AbstractSxTree) Sort(str []string) []string {
	k, v := abs.GetKeys()
	for arrIdx := range str {
		for i := range k {
			if str[arrIdx] == k[i] {
				str[arrIdx] = v[i]
			}
		}
		//kekich := fmt.Sprintf("str:%s", str[arrIdx])
		//lol2.Printf(kekich)

	}
	//for i := range k {
	//keka := fmt.Sprintf("k:%s v:%s", k[i], v[i])
	//lol.Printf(keka)
	//}
	return str
}

func (abs *AbstractSxTree) CombineIntoPath(imprs []*ast.ImportSpec) {
	for i := range imprs {
		if imprs[i].Doc != nil && imprs[i].Name == nil {
			commentedPath := fmt.Sprintf(`//%s    %s`, imprs[i].Doc.Text(), imprs[i].Path.Value)
			abs.advanced[imprs[i].Path.Value] = commentedPath
		}
		if imprs[i].Doc == nil && imprs[i].Name != nil {
			namedPath := fmt.Sprintf("%s %s", imprs[i].Name.String(), imprs[i].Path.Value)
			abs.advanced[imprs[i].Path.Value] = namedPath
		}
		if imprs[i].Doc != nil && imprs[i].Name != nil {
			commentedNamedPath := fmt.Sprintf(`//%s    %s%s`, imprs[i].Doc.Text(), imprs[i].Name.String(), imprs[i].Path.Value)
			abs.advanced[imprs[i].Path.Value] = commentedNamedPath
		}
		if imprs[i].Doc == nil && imprs[i].Name == nil {
			abs.advanced[imprs[i].Path.Value] = imprs[i].Path.Value
		}
	}
}

var cfg = printer.Config{Mode: printer.RawFormat, Tabwidth: 0}

func ImportCheck(file string) bool {
	fset := token.NewFileSet() // positions are relative to fset

	src, _ := os.ReadFile(file)

	f, _ := parser.ParseFile(fset, "", src, parser.ImportsOnly)

	if len(f.Imports) > 1 {
		return true
	}

	return false
}

type ImportsArr struct {
	paths, comments, names []string
}

func (abs *AbstractSxTree) getImports(file string) ([]string, error) {
	//fset := token.NewFileSet() // positions are relative to fset
	src, err := os.ReadFile(file)

	err = abs.NewTree(src, file)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	abs.astFile, err = parser.ParseFile(abs.fset, "", src, parser.ImportsOnly)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	imports := make([]string, len(abs.astFile.Imports))
	for i, s := range abs.astFile.Imports {
		imports[i] = s.Path.Value
	}
	for i := range imports {
		imports[i], err = strconv.Unquote(imports[i])
		if err != nil {
			return nil, fmt.Errorf("%w", err)
		}
	}
	return imports, nil
}

func (abs *AbstractSxTree) MergeImportsSlices(file string) (std []string) {
	imports, _ := abs.getImports(file)

	std = SortImportsByRank(imports, 0)
	std = append(std, "\n")
	ext := SortImportsByRank(imports, 1)
	ext = append(ext, "\n")
	std = append(std, ext...)

	out := SortImportsByRank(imports, 2)
	out = append(out, "\n")

	proj := SortImportsByRank(imports, 3)
	proj = append(proj, "\n")
	out = append(out, proj...)
	std = append(std, out...)

	return
}
func (abs *AbstractSxTree) WriteImports(b []byte, str []string, file string) error {
	err := abs.NewTree(b, file)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	abs.astFile, err = parser.ParseFile(abs.fset, "", b, parser.ImportsOnly)
	if err != nil {
		log.Fatalln(err)
	}

	for i := 0; i < len(abs.astFile.Decls); i++ {
		d := abs.astFile.Decls[i]
		switch d.(type) {
		case *ast.FuncDecl:
		case *ast.GenDecl:

			dd := d.(*ast.GenDecl)

			if dd.Tok == token.IMPORT {
				for ii := range str {
					if str[ii] == "\n" {
						iSpec := &ast.ImportSpec{Path: &ast.BasicLit{Value: str[ii]}}
						dd.Specs = append(dd.Specs, iSpec)
					} else {
						iSpec := &ast.ImportSpec{Path: &ast.BasicLit{Value: str[ii]}}
						dd.Specs = append(dd.Specs, iSpec)
					}

				}
			}
		}
	}

	var buf bytes.Buffer
	out, _ := format(abs.fset, abs.astFile, abs.sourceAdj, abs.indentAdj, b, cfg)

	buf.Write(out)
	lol.Println(string(buf.Bytes()))

	return nil
}
func SortImportsByRank(importsFromFile []string, rank int) []string {
	var rankedList []string
	for pkg, ii := range package_collector.Packages {
		for i := range importsFromFile {
			//rank = package_collector.Packages[pkg]
			if pkg == importsFromFile[i] && ii == rank {
				rankedList = append(rankedList, pkg)
			}
		}
	}

	return rankedList
}
