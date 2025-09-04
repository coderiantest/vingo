// vingo_cli.go
package vingo

import (
	"fmt"
	"os"
	"path/filepath"
)

func Init() {
	args := os.Args
	if len(args) < 2 || args[1] != "init" {
		fmt.Println("Kullanım: vingo init")
		return
	}

	// Mevcut dizin
	home, err := os.Getwd()
	if err != nil {
		fmt.Println("Dizin alınamadı:", err)
		os.Exit(1)
	}

	// settings.json yolu
	settingsPath := filepath.Join(home, "settings.json")

	// İçerik
	content := `{
  "fileAssociations": [
    {
      "extension": ".vgo",
      "icon": "html",
      "syntax": "html"
    },
    {
      "extension": ".vingo",
      "icon": "html",
      "syntax": "html"
    }
  ]
}`

	// Dosyayı yaz
	err = os.WriteFile(settingsPath, []byte(content), 0644)
	if err != nil {
		fmt.Println("Dosya oluşturulamadı:", err)
		os.Exit(1)
	}

	fmt.Println("✅ settings.json oluşturuldu ve içine fileAssociations eklendi")
}
