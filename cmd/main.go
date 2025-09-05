package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Kullanım: vingo <komut>")
		return
	}

	switch os.Args[1] {
	case "create":
		dir := ".vscode"
		file := filepath.Join(dir, "settings.json")

		// Klasörü oluştur
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Println("Klasör oluşturulamadı:", err)
			return
		}

		// JSON içeriği
		content := `{
    "files.associations": {
        "*.vgo": "html"
    }
}`

		// Dosyayı yaz
		if err := os.WriteFile(file, []byte(content), 0644); err != nil {
			fmt.Println("Dosya yazılamadı:", err)
			return
		}

		fmt.Println(".vscode/settings.json başarıyla oluşturuldu ✅")

	default:
		fmt.Println("Bilinmeyen komut:", os.Args[1])
	}
}
