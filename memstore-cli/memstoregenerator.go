package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"text/template"

	"github.com/orktes/go-memstore/parser"
)

var buildTags = flag.String("build_tags", "", "build tags to add to generated file")
var allStructs = flag.Bool("all", false, "generate marshaler/unmarshalers for all structs in a file")
var specifiedName = flag.String("output_filename", "", "specify the filename of the output")

var fns = template.FuncMap{
	"last": func(x int, a interface{}) bool {
		return x == reflect.ValueOf(a).Len()-1
	},
}

var tmpl = template.Must(template.New("generated_file").Funcs(fns).Parse(`
	package {{.PkgName}}

	// NOTE: This file is generated by memstore. Do not edit!

	import (
		"strconv"
		"strings"

		"github.com/orktes/go-memstore"
	)

	{{ define "struct" }}

	func memStoreGenerate{{.StructName}}Key(s {{.StructName}}) string {
		var b strings.Builder

		{{ range $index, $field := .Fields }}
			{{ if $field.Index }}
				{{ if ne $index 0 }}
				b.Write([]byte(":"))
				{{ end }}

				{{ if eq $field.Type "string" }}
				b.WriteString(s.{{$field.Name}})
				{{ else if eq $field.Type "bool" }}
				b.WriteString(strconv.FormatBool(s.{{$field.Name}}))
				{{ else if eq $field.Type "int" }}
				b.WriteString(strconv.FormatInt(int64(s.{{$field.Name}}), 10))
				{{ else if eq $field.Type "int32" }}
				b.WriteString(strconv.FormatInt(int64(s.{{$field.Name}}), 10))
				{{ else if eq $field.Type "int64" }}
				b.WriteString(strconv.FormatInt(s.{{$field.Name}}, 10))
				{{ else if eq $field.Type "uint" }}
				b.WriteString(strconv.FormatUInt(uint64(s.{{$field.Name}}), 10))
				{{ else if eq $field.Type "uint32" }}
				b.WriteString(strconv.FormatUInt(uint64(s.{{$field.Name}}), 10))
				{{ else if eq $field.Type "uint64" }}
				b.WriteString(strconv.FormatUInt(s.{{$field.Name}}, 10))
				{{ else if eq $field.Type "float64" }}
				b.WriteString(strconv.FormatFloat(s.{{$field.Name}}, 'E', -1, 64))
				{{ else if eq $field.Type "float32" }}
				b.WriteString(strconv.FormatFloat(float64(s.{{$field.Name}}), 'E', -1, 32))
				{{ else }}
				b.WriteString(fmt.Sprintf("%s", s.{{$field.Name}}))
				{{end}}
			{{end}}
		{{end}}

		return b.String()
	}

	type stored{{.StructName}} struct {
		{{ range $index, $value := .Fields }}{{if not $value.Index }}{{$value.Name}} {{$value.Type}}{{end}}
		{{ end }}
	}

	// {{.StructName}}MemStoreQuery is used to query data from the memory store
	type {{.StructName}}MemStoreQuery struct {
		{{ range $index, $value := .Fields }}{{if $value.Index }}{{$value.Name}} {{$value.Type}}{{end}}
		{{ end }}
	}

	// {{.StructName}}MemStore instance of a memory store for {{.StructName}}
	type {{.StructName}}MemStore struct {
		store *memstore.Store
	}

	// New{{.StructName}}MemStore creates a new memorystore for {{.StructName}} struct instances
	func New{{.StructName}}MemStore() *{{.StructName}}MemStore {
		return &{{.StructName}}MemStore{
			store: memstore.New(),
		}
	}

	// Insert inserts an instance of {{.StructName}} to the memorystore
	func (s *{{.StructName}}MemStore) Insert(i {{.StructName}}) {
		stored := stored{{.StructName}}{}

		{{ range $index, $value := .Fields }}{{if not $value.Index }}stored.{{$value.Name}} = i.{{$value.Name}}{{end}}
		{{ end }}

		key := memStoreGenerate{{.StructName}}Key(i)

		s.store.Insert(key, stored)
	}

	// Get returns an instance of {{.StructName}} from the memorystore
	func (s *{{.StructName}}MemStore) Get(query {{.StructName}}MemStoreQuery) (res {{.StructName}}, ok bool)  {
		{{ range $index, $value := .Fields }}{{if $value.Index }}res.{{$value.Name}} = query.{{$value.Name}}{{end}}
		{{ end }}

		key := memStoreGenerate{{.StructName}}Key(res)

		val, ok := s.store.Get(key)
		if !ok {
			return res, false
		}

		stored := val.(stored{{.StructName}})
		{{ range $index, $value := .Fields }}{{if not $value.Index }}res.{{$value.Name}} = stored.{{$value.Name}}{{end}}
		{{ end }}

		return
	}

	{{ end }}

	{{ range $key, $value := .Structs }}
	{{ template "struct" $value }}
	{{ end }}
	
`))

func writeGeneratedFile(output string, p parser.Parser) (err error) {
	f, err := os.Create(output)
	if err != nil {
		return err
	}

	if err := tmpl.Execute(f, p); err != nil {
		f.Close()
		return err
	}

	f.Close()

	cmd := exec.Command("gofmt", "-w", f.Name())
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err = cmd.Run(); err != nil {
		return err
	}

	return nil
}

func generate(fname string) (err error) {
	fInfo, err := os.Stat(fname)
	if err != nil {
		return err
	}

	p := parser.Parser{AllStructs: *allStructs}
	if err := p.Parse(fname, fInfo.IsDir()); err != nil {
		return fmt.Errorf("Error parsing %v: %v", fname, err)
	}

	var outName string
	if fInfo.IsDir() {
		outName = filepath.Join(fname, p.PkgName+"_memstore_gen.go")
	} else {
		if s := strings.TrimSuffix(fname, ".go"); s == fname {
			return errors.New("Filename must end in '.go'")
		} else {
			outName = s + "_memstore.go"
		}
	}

	if *specifiedName != "" {
		outName = *specifiedName
	}

	return writeGeneratedFile(outName, p)
}

func main() {
	flag.Parse()
	files := flag.Args()

	gofile := os.Getenv("GOFILE")

	if gofile != "" {
		files = append(files, gofile)
	}

	for _, file := range files {
		if err := generate(file); err != nil {
			panic(err)
		}
	}

}
