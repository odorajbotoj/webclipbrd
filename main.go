// main.go
package main

import (
	"flag"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

const HTML = `<!DOCTYPE HTML>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=devie-width, initial-scale=1.0">
<meta http-equiv="X-UA-Compatible" content="ie=edge">
<title>web clip board</title>
</head>
<body>
<h1>web clip board</h1>
<hr />
<fieldset>
<legend>Clip board</legend>
<ul>
{{range $k,$v := .Text}}
<li>{{$v}}&nbsp;&nbsp;&nbsp;&nbsp;<a href="del?k={{$k}}" onclick="return confirm('Delete?');">[Del]</a></li>
{{end}}
<hr />
<li>
<form action="/" method="post">
<input type="text" name="text">
<input type="submit" value="Send"/>
</form>
</li>
</ul>
</fieldset>
<fieldset>
<legned>File transfer</legned>
<ul>
{{range $name,$size := .Files}}
<li><a href="/getf?fn={{$name}}" target="_blank">{{$name}}</a>&nbsp;&nbsp;({{$size}} Byte)&nbsp;&nbsp;&nbsp;&nbsp;<a href="/delf?fn={{$name}}" onclick="return confirm('Delete {{$name}} ?');">[Del]</a></li>
{{end}}
<hr />
<li>
<form method="post" action="/upldf" enctype="multipart/form-data">
<label> Note: &lt; 100 MB </label>
<input type="file" id="file" name="file" multiple />
<input type="submit" value="Upload" />
</form>
</li>
</ul>
</fieldset>
</body>`

var clipbrd map[int64]string

func getSend(d string) map[string]int64 {
	var ret = make(map[string]int64)
	rd, err := os.ReadDir(d)
	if err != nil {
		log.Println("getSend: ", err)
		return ret
	}
	for _, fi := range rd {
		if !fi.IsDir() {
			info, err := fi.Info()
			if err != nil {
				continue
			}
			ret[info.Name()] = info.Size()
		}
	}
	return ret
}

func main() {
	argDir := flag.String("dir", "files/", "file dir")
	argPort := flag.String("port", ":8081", "server port")
	flag.Parse()
	os.RemoveAll(*argDir)
	os.MkdirAll(*argDir, 0664)
	clipbrd = make(map[int64]string)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			t, _ := template.New("index").Parse(HTML)
			t.Execute(w, struct {
				Text  map[int64]string
				Files map[string]int64
			}{clipbrd, getSend(*argDir)})
		} else if r.Method == "POST" {
			if r.FormValue("text") != "" {
				clipbrd[time.Now().UnixMilli()] = r.FormValue("text")
			}
			http.Redirect(w, r, "/", http.StatusSeeOther)
		} else {
			return
		}
	})
	http.HandleFunc("/del", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			defer func() {
				if err := recover(); err != nil {
					log.Println(err)
					http.Redirect(w, r, "/", http.StatusSeeOther)
					return
				}
			}()
			k := r.URL.Query().Get("k")
			ind, _ := strconv.ParseInt(k, 10, 64)
			delete(clipbrd, ind)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	})
	http.HandleFunc("/upldf", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 100*1024*1024+1024)
		if err := r.ParseMultipartForm(100*1024*1024 + 1024); err != nil {
			http.Error(w, "The uploaded file is too big. Please choose an file that's less than 100MB in size", http.StatusBadRequest)
			return
		}
		files, ok := r.MultipartForm.File["file"]
		if !ok { // 出错则取消
			http.Error(w, "UnKnown Error. Please retry.", http.StatusBadRequest)
			return
		}
		for _, f := range files {
			fr, _ := f.Open()
			fo, err := os.Create(*argDir + f.Filename)
			defer fr.Close()
			defer fo.Close()
			if err != nil {
				log.Println("upldf: ", err)
				continue
			}
			io.Copy(fo, fr)
			log.Println("uploaded '" + f.Filename + "'")
		}
		http.Redirect(w, r, "/", http.StatusFound)
		return
	})
	http.HandleFunc("/delf", func(w http.ResponseWriter, r *http.Request) {
		fn := r.URL.Query().Get("fn")
		if fn != "" {
			err := os.Remove(*argDir + fn)
			if err != nil {
				log.Println("delf: ", err)
			}
			log.Println("deleted '" + fn + "'")
		}
		http.Redirect(w, r, "/", http.StatusFound)
	})
	http.HandleFunc("/getf", func(w http.ResponseWriter, r *http.Request) {
		fn := r.URL.Query().Get("fn")
		if fn == "" {
			http.Error(w, "404. File not found.", http.StatusNotFound)
		} else {
			b, err := os.ReadFile(*argDir + fn)
			if err != nil {
				log.Println("/getf: ", err)
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			finfo, _ := os.Stat(*argDir + fn)
			w.Header().Set("Content-Disposition", "attachment; filename="+fn)
			w.Header().Set("Content-Type", http.DetectContentType(b))
			w.Header().Set("Content-Length", strconv.FormatInt(finfo.Size(), 10))
			w.Write(b)
			return
		}
	})
	log.Fatalln(http.ListenAndServe(*argPort, nil))
}
