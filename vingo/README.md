<p align="center">
  <img src="assets/vingo.png" alt="Vingo" width="200"/>
</p>

## ⚡️ What is Vingo?
**Vingo** is a modern template engine developed
under the [Flint Framework](https://github.com/coderianx/flint). it provides a fast, flexible, and clean way to render templates in Golang,with full entegration into Flint.

---

## ⚙️ Installation
```bash
go get github.com/coderianx/vingo
```

## 🚀 Quick Start

```go
package main

import (
	"net/http"
    
    "github.com/coderianx/vingo"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

        tmpl, err := vingo.Render("index.html", map[string]interface{}{
            "title": "Welcome to Vingo",
            "message": "Hello, Vingo!",
        })

		w.Write([]byte(html))
	})

	http.ListenAndServe(":8080", nil)
}
````
### Template Example (index.html)
```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title><{ title }></title>
</head>
<body>
    <h1><{ message }></h1>
</body>
</html>
```
## 📄 License
This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.