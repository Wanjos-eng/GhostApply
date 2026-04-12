package main

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/Wanjos-eng/GhostApply/internal/domain"
	"github.com/Wanjos-eng/GhostApply/internal/infra/llm"
	"github.com/Wanjos-eng/GhostApply/internal/infra/pw"
	"github.com/playwright-community/playwright-go"
)

// processApplication orchestrates the automation of Tasks 40-49.
func processApplication(ctx playwright.BrowserContext, database *sql.DB, groqClient *llm.GroqClient, c domain.VagaComCandidatura) error {
	page, err := ctx.NewPage()
	if err != nil {
		return err
	}
	defer page.Close()

	if _, err := page.Goto(c.Vaga.URL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return fmt.Errorf("Task 40: failed to navigate: %w", err)
	}

	pw.HumanSleep()

	// Task 41: Click 'Easy Apply' (usually jobs-apply-button)
	// We'll use a broad text locator for "Easy Apply" or "Apply now"
	applyBtn := page.Locator("button:has-text('Easy Apply')").First()
	btnCount, err := applyBtn.Count()
	if err != nil || btnCount == 0 {
		return fmt.Errorf("Task 41: easy apply button not found (perhaps already applied or external layout changes)")
	}

	if err := applyBtn.Click(); err != nil {
		return fmt.Errorf("Task 41: failed to click Easy Apply: %w", err)
	}

	// Pagination loop (Tasks 42-48)
	pageLimit := 10 // Safe guard preventing infinite loops
	for i := 0; i < pageLimit; i++ {
		pw.HumanSleep()

		// If we see Application sent overlay, break loop early
		postApplyConf := page.Locator("text='Application sent'")
		if cnt, _ := postApplyConf.Count(); cnt > 0 {
			break
		}

		// Fill normal text inputs
		if err := fillTextInputs(page, groqClient); err != nil {
			log.Printf("filler warning: failed mapping text inputs: %v", err)
		}

		// Handle File Uploads (Task 47)
		if err := handleFileUpload(page, c.Candidatura.CurriculoPath); err != nil {
			log.Printf("filler warning: failed attaching file: %v", err)
		}

		// Handle Next / Format / Review / Submit
		nextBtn := page.Locator("button:has-text('Next')").First()
		reviewBtn := page.Locator("button:has-text('Review')").First()
		submitBtn := page.Locator("button:has-text('Submit application')").First()

		if ok, _ := submitBtn.IsVisible(); ok {
			// Task 48: Click Submit!
			if err := submitBtn.Click(); err != nil {
				return fmt.Errorf("Task 48: failed to submit: %w", err)
			}
			pw.HumanSleep()
			break
		} else if ok, _ := reviewBtn.IsVisible(); ok {
			if err := reviewBtn.Click(); err != nil {
				log.Printf("filler warn: failed to click review")
			}
		} else if ok, _ := nextBtn.IsVisible(); ok {
			if err := nextBtn.Click(); err != nil {
				log.Printf("filler warn: failed to click next")
			}
		} else {
			// No standard buttons, it's either an arbitrary page load or an unexpected state
			pw.HumanSleep()
		}
	}

	pw.HumanSleep()
	
	// Task 49: Validate "Application sent" successful modal text
	successText := page.Locator("text='Application sent'")
	if cnt, _ := successText.Count(); cnt == 0 {
		return fmt.Errorf("Task 49: could not verify application success state")
	}

	return nil
}

func fillTextInputs(page playwright.Page, groqClient *llm.GroqClient) error {
	// Find all standard input textfields or textareas internally
	inputs := page.Locator("input[type='text'], textarea, input[type='number'], input[type='tel']")
	count, err := inputs.Count()
	if err != nil {
		return err
	}

	for i := 0; i < count; i++ {
		inputLoc := inputs.Nth(i)
		
		// If already filled, skip
		val, _ := inputLoc.InputValue()
		if val != "" {
			continue
		}

		// Identify Label text (Task 43)
		var labelText string
		id, _ := inputLoc.GetAttribute("id")
		if id != "" {
			labelLoc := page.Locator(fmt.Sprintf("label[for='%s']", id))
			if visible, _ := labelLoc.IsVisible(); visible {
				labelText, _ = labelLoc.InnerText()
			}
		}

		if labelText == "" {
			// Try implicit label
			labelLoc := page.Locator("label").Filter(playwright.LocatorFilterOptions{Has: inputLoc})
			if visible, _ := labelLoc.IsVisible(); visible {
				labelText, _ = labelLoc.InnerText()
			}
		}

		// Task 44: known label OR LLM
		answer := getAnswerForLabel(labelText, groqClient)
		if answer != "" {
			// Task 45: TypeHumanly simulator
			if err := pw.TypeHumanly(inputLoc, answer); err != nil {
				log.Printf("TypeHumanly error: %v", err)
			}
		}
	}
	return nil
}

func handleFileUpload(page playwright.Page, resumePath string) error {
	// Task 47
	fileInput := page.Locator("input[type='file']").First()
	isVis, _ := fileInput.IsVisible()
	
	// Native Playwright handling: The element might be hidden, so we attach directly.
	count, _ := fileInput.Count()
	if count > 0 && isVis {
		err := fileInput.SetInputFiles([]string{resumePath})
		if err != nil {
			return err
		}
		log.Printf("Resume successfully loaded: %s", resumePath)
	}
	return nil
}

func getAnswerForLabel(label string, groqClient *llm.GroqClient) string {
	lowerLabel := strings.ToLower(label)
	
	// Base standard deterministic resolution
	if match, _ := regexp.MatchString(`.*(phone|telefone|celular).*`, lowerLabel); match {
		return getEnv("USER_PHONE", "+551199999999")
	}
	if match, _ := regexp.MatchString(`.*(city|cidade|reside).*`, lowerLabel); match {
		return getEnv("USER_CITY", "São Paulo")
	}
	
	// For complex fields, route to Groq
	profileContext := getEnv("USER_PROFILE_CONTEXT", "Backend developer experienced in Go, Rust, React. Lives in Brazil.")
	
	ans, err := groqClient.AnswerFormField(label, profileContext)
	if err != nil {
		log.Printf("groq warn: failed to answer question '%s': %v", label, err)
		return ""
	}
	return ans
}
