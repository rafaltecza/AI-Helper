package main

import (
	"bytes"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/atotto/clipboard"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

type FileExplorer struct {
	app           fyne.App
	currentDir    string
	selectedFiles map[string]bool
	prefix        string
	window        fyne.Window
}

func NewFileExplorer(app fyne.App, dir, prefix string) *FileExplorer {
	return &FileExplorer{
		app:           app,
		currentDir:    dir,
		selectedFiles: make(map[string]bool),
		prefix:        prefix,
		window:        app.NewWindow("File Explorer"),
	}
}

func (fe *FileExplorer) updateContent() fyne.CanvasObject {
	// Read directory contents
	entries, err := os.ReadDir(fe.currentDir)
	if err != nil {
		return widget.NewLabel(fmt.Sprintf("Error reading directory: %v", err))
	}

	// Sort entries (folders first, then files)
	var folders, files []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			folders = append(folders, entry)
		} else if fe.prefix == "" || (fe.prefix != "" && hasPrefix(entry.Name(), fe.prefix)) {
			files = append(files, entry)
		}
	}
	sort.Slice(folders, func(i, j int) bool { return folders[i].Name() < folders[j].Name() })
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })

	// Create tiles
	tiles := container.NewVBox()

	// Add ".." for parent directory if not at root
	rowIndex := 0
	if !isRoot(fe.currentDir) {
		row := fe.createParentNavigationRow("..")
		bg := canvas.NewRectangle(theme.Color(theme.ColorNameBackground))
		if rowIndex%2 == 0 {
			bg.FillColor = theme.Color(theme.ColorNameBackground)
		} else {
			bg.FillColor = theme.Color(theme.ColorNameHover)
		}
		tiles.Add(container.NewStack(bg, row))
		rowIndex++
	}

	// Add folders (now with checkbox to allow selection)
	for _, folder := range folders {
		folderPath := filepath.Join(fe.currentDir, folder.Name())

		check := widget.NewCheck(folder.Name(), func(checked bool) {
			fe.selectedFiles[folderPath] = checked
		})
		// Odtwórz stan checkboxa po rerenderze
		if v, ok := fe.selectedFiles[folderPath]; ok {
			check.SetChecked(v)
		}

		row := container.NewHBox(
			widget.NewIcon(theme.FolderIcon()),
			check,
			layout.NewSpacer(), // Push buttons to the right
			// Otwórz w tym oknie
			widget.NewButtonWithIcon("", theme.ViewFullScreenIcon(), func() {
				fe.currentDir = folderPath
				fe.selectedFiles = make(map[string]bool) // Clear selections
				fe.window.SetContent(fe.updateContent())
			}),
			// Otwórz w nowym oknie
			widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
				newExplorer := NewFileExplorer(fe.app, folderPath, fe.prefix)
				newExplorer.window.SetContent(newExplorer.updateContent())
				newExplorer.window.Show()
			}),
		)

		bg := canvas.NewRectangle(theme.Color(theme.ColorNameBackground))
		if rowIndex%2 == 0 {
			bg.FillColor = theme.Color(theme.ColorNameBackground)
		} else {
			bg.FillColor = theme.Color(theme.ColorNameHover)
		}
		tiles.Add(container.NewStack(bg, row))
		rowIndex++
	}

	// Add files
	for _, file := range files {
		filePath := filepath.Join(fe.currentDir, file.Name())
		check := widget.NewCheck(file.Name(), func(checked bool) {
			fe.selectedFiles[filePath] = checked
		})
		// Odtwórz stan checkboxa po rerenderze
		if v, ok := fe.selectedFiles[filePath]; ok {
			check.SetChecked(v)
		}

		row := container.NewHBox(
			widget.NewIcon(theme.FileIcon()),
			check,
			layout.NewSpacer(), // Push buttons to the right
			widget.NewButtonWithIcon("", theme.ViewFullScreenIcon(), func() { fe.openFile(filePath, false) }),
			widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() { fe.openFile(filePath, true) }),
		)
		bg := canvas.NewRectangle(theme.Color(theme.ColorNameBackground))
		if rowIndex%2 == 0 {
			bg.FillColor = theme.Color(theme.ColorNameBackground)
		} else {
			bg.FillColor = theme.Color(theme.ColorNameHover)
		}
		tiles.Add(container.NewStack(bg, row))
		rowIndex++
	}

	// Create buttons
	selectAll := widget.NewButton("Select All", func() {
		// Zaznacz widoczne foldery i pliki
		for _, folder := range folders {
			folderPath := filepath.Join(fe.currentDir, folder.Name())
			fe.selectedFiles[folderPath] = true
		}
		for _, file := range files {
			filePath := filepath.Join(fe.currentDir, file.Name())
			fe.selectedFiles[filePath] = true
		}
		// Prostszym i stabilnym sposobem jest odświeżenie widoku,
		// aby checkboxy pobrały stan z mapy selectedFiles.
		fe.window.SetContent(fe.updateContent())
	})

	copyButton := widget.NewButton("Copy Selected", func() { fe.copySelected() })
	currentPath := widget.NewLabel(fe.currentDir)

	// Wrap tiles in a vertical scroll container
	scrollableContent := container.NewVScroll(tiles)

	// Main content with scroll
	content := container.NewBorder(
		currentPath,
		container.NewHBox(selectAll, copyButton),
		nil,
		nil,
		scrollableContent,
	)

	// Set an initial size, but allow resizing
	fe.window.Resize(fyne.NewSize(600, 400))

	return content
}

// Wiersz nawigacyjny dla ".."
func (fe *FileExplorer) createParentNavigationRow(name string) *fyne.Container {
	return container.NewHBox(
		widget.NewIcon(theme.FolderIcon()),
		widget.NewButton(name, func() {
			parent := filepath.Dir(fe.currentDir)
			fe.currentDir = parent
			fe.selectedFiles = make(map[string]bool) // Clear selections
			fe.window.SetContent(fe.updateContent())
		}),
		layout.NewSpacer(),
		widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
			parent := filepath.Dir(fe.currentDir)
			newExplorer := NewFileExplorer(fe.app, parent, fe.prefix)
			newExplorer.window.SetContent(newExplorer.updateContent())
			newExplorer.window.Show()
		}),
	)
}

func (fe *FileExplorer) openFile(filePath string, newWindow bool) {
	info, err := os.Stat(filePath)
	if err != nil {
		fe.showError(fmt.Sprintf("Error accessing file: %v", err))
		return
	}

	if info.IsDir() {
		if newWindow {
			newExplorer := NewFileExplorer(fe.app, filePath, fe.prefix)
			newExplorer.window.SetContent(newExplorer.updateContent())
			newExplorer.window.Show()
		} else {
			fe.currentDir = filePath
			fe.selectedFiles = make(map[string]bool)
			fe.window.SetContent(fe.updateContent())
		}
	} else {
		// Handle file opening
		content, err := os.ReadFile(filePath)
		if err != nil {
			fe.showError(fmt.Sprintf("Error reading file: %v", err))
			return
		}

		var w fyne.Window
		if newWindow {
			w = fe.app.NewWindow(filepath.Base(filePath))
			w.SetOnClosed(func() {
				// Ensure the window is properly closed
				w.Close()
			})
		} else {
			w = fe.window
		}

		// Try to display as image first
		img := canvas.NewImageFromFile(filePath)
		img.FillMode = canvas.ImageFillContain
		img.SetMinSize(fyne.NewSize(400, 300))

		// If not an image, display as text
		if img.Resource == nil {
			text := widget.NewLabel(string(content))
			text.Wrapping = fyne.TextWrapWord
			w.SetContent(container.NewVScroll(text))
		} else {
			w.SetContent(container.NewVScroll(img))
		}

		if newWindow {
			w.Resize(fyne.NewSize(600, 400))
			w.Show()
		}
	}
}

func (fe *FileExplorer) copySelected() {
	var buffer bytes.Buffer
	copiedBlocks := 0

	// Przejdź po wszystkich zaznaczonych ścieżkach
	for path, selected := range fe.selectedFiles {
		if !selected {
			continue
		}

		info, err := os.Stat(path)
		if err != nil {
			fe.showError(fmt.Sprintf("Error accessing %s: %v", path, err))
			continue
		}

		if info.IsDir() {
			// Rekurencyjnie dodaj wszystkie pliki z katalogu
			err := filepath.WalkDir(path, func(p string, d fs.DirEntry, werr error) error {
				if werr != nil {
					return nil // pomiń problematyczne ścieżki
				}
				if d.IsDir() {
					return nil
				}
				// Wyświetlaj nagłówek z relatywną ścieżką (względem root zaznaczonego folderu)
				rel, rerr := filepath.Rel(path, p)
				display := rel
				if rerr != nil || rel == "." {
					display = filepath.Base(p)
				}
				if fe.appendFileContent(&buffer, p, display) {
					copiedBlocks++
				}
				return nil
			})
			if err != nil {
				fe.showError(fmt.Sprintf("Error walking directory %s: %v", path, err))
			}
		} else {
			// Pojedynczy plik
			display := filepath.Base(path)
			if fe.appendFileContent(&buffer, path, display) {
				copiedBlocks++
			}
		}
	}

	if copiedBlocks == 0 {
		fe.showError("No files selected")
		return
	}

	if err := clipboard.WriteAll(buffer.String()); err != nil {
		fe.showError(fmt.Sprintf("Failed to copy to clipboard: %v", err))
		return
	}

	fyne.CurrentApp().SendNotification(&fyne.Notification{
		Title:   "Success",
		Content: fmt.Sprintf("Copied %d file blocks to clipboard", copiedBlocks),
	})
}

// Zwraca true, jeśli udało się dodać blok (plik) do bufora
func (fe *FileExplorer) appendFileContent(buffer *bytes.Buffer, filePath, displayName string) bool {
	content, err := os.ReadFile(filePath)
	if err != nil {
		fe.showError(fmt.Sprintf("Error reading file %s: %v", filePath, err))
		return false
	}
	// Nagłówek
	buffer.WriteString(fmt.Sprintf("--- %s ---\n", displayName))
	// Zawartość
	buffer.Write(content)
	// Separator linii po każdym pliku
	buffer.WriteString("\n")
	return true
}

func (fe *FileExplorer) showError(message string) {
	fyne.CurrentApp().SendNotification(&fyne.Notification{
		Title:   "Error",
		Content: message,
	})
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func isRoot(path string) bool {
	return path == "/" || (len(path) == 3 && path[1] == ':' && path[2] == '\\')
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: program <path> [prefix]")
		os.Exit(1)
	}

	dir := os.Args[1]
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		fmt.Printf("Error: %s is not a valid directory\n", dir)
		os.Exit(1)
	}

	prefix := ""
	if len(os.Args) > 2 {
		prefix = os.Args[2]
	}

	// Create a single app instance
	a := app.New()
	explorer := NewFileExplorer(a, dir, prefix)
	explorer.window.SetContent(explorer.updateContent())
	explorer.window.ShowAndRun()
}
