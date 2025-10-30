package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"
)

var templates *template.Template

func init() {
	// Load all templates
	var err error
	templates, err = template.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatal("Failed to load templates:", err)
	}
}

func main() {
	// Home page
	http.HandleFunc("/", handleHome)

	// Test scenarios
	http.HandleFunc("/instant", handleInstant)
	http.HandleFunc("/slow-resource", handleSlowResource)
	http.HandleFunc("/form", handleForm)
	http.HandleFunc("/navigation", handleNavigation)
	http.HandleFunc("/spa", handleSPA)
	http.HandleFunc("/multiple-requests", handleMultipleRequests)
	http.HandleFunc("/diff-test", handleDiffTest)
	http.HandleFunc("/nested-interactive", handleNestedInteractive)
	http.HandleFunc("/accessibility-test", handleAccessibilityTest)
	http.HandleFunc("/kakao-test", handleKakaoTest)
	http.HandleFunc("/dialog-test", handleDialogTest)

	// Resource endpoints for testing
	http.HandleFunc("/delay", handleDelay)
	http.HandleFunc("/image.png", handleImage)

	port := ":8080"
	log.Printf("Test server starting on http://localhost%s", port)
	log.Printf("Open http://localhost:8080 in wb to see test scenarios")
	log.Fatal(http.ListenAndServe(port, nil))
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "home.html", nil)
}

func handleInstant(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "instant.html", nil)
}

func handleSlowResource(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "slow-resource.html", nil)
}

func handleForm(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Message    string
		Username   string
		Email      string
		Bio        string
		Country    string
		Newsletter string
		Gender     string
	}{}

	if r.Method == "POST" {
		r.ParseForm()
		data.Username = r.FormValue("username")
		data.Email = r.FormValue("email")
		data.Bio = r.FormValue("bio")
		data.Country = r.FormValue("country")
		data.Newsletter = r.FormValue("newsletter")
		data.Gender = r.FormValue("gender")

		data.Message = fmt.Sprintf("Form submitted! Username: %s, Email: %s, Country: %s, Newsletter: %s, Gender: %s",
			data.Username, data.Email, data.Country, data.Newsletter, data.Gender)
	}

	templates.ExecuteTemplate(w, "form.html", data)
}

func handleNavigation(w http.ResponseWriter, r *http.Request) {
	page := r.URL.Query().Get("page")
	if page == "" {
		page = "1"
	}

	data := struct {
		Page string
	}{
		Page: page,
	}

	templates.ExecuteTemplate(w, "navigation.html", data)
}

func handleSPA(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "spa.html", nil)
}

func handleMultipleRequests(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "multiple-requests.html", nil)
}

func handleDiffTest(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "diff-test.html", nil)
}

func handleNestedInteractive(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "nested-interactive.html", nil)
}

func handleAccessibilityTest(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "accessibility-test.html", nil)
}

func handleKakaoTest(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "kakao-test.html", nil)
}

func handleDialogTest(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "dialog-test.html", nil)
}

// Helper endpoints

func handleDelay(w http.ResponseWriter, r *http.Request) {
	secondsStr := r.URL.Query().Get("seconds")
	seconds, err := strconv.Atoi(secondsStr)
	if err != nil || seconds < 0 || seconds > 10 {
		seconds = 1
	}

	time.Sleep(time.Duration(seconds) * time.Second)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprintf(`{"delayed": %d, "timestamp": "%s"}`, seconds, time.Now().Format(time.RFC3339))))
}

func handleImage(w http.ResponseWriter, r *http.Request) {
	// 1x1 transparent PNG
	png := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
		0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
		0x42, 0x60, 0x82,
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(png)
}
