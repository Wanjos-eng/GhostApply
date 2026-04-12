package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/Wanjos-eng/GhostApply/internal/domain"
	"github.com/Wanjos-eng/GhostApply/internal/infra/llm"
	"github.com/Wanjos-eng/GhostApply/internal/infra/pw"
	"github.com/playwright-community/playwright-go"
)

// processApplication executa o fluxo de candidatura automatizada para uma vaga.
func processApplication(ctx playwright.BrowserContext, groqClient *llm.GroqClient, c domain.VagaComCandidatura) error {
	page, err := ctx.NewPage()
	if err != nil {
		return err
	}
	defer page.Close()

	if _, err := page.Goto(c.Vaga.URL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return fmt.Errorf("falha ao navegar até a vaga: %w", err)
	}

	pw.HumanSleep()

	// Tarefa 41: clica em Easy Apply (normalmente jobs-apply-button).
	// Usa um locator amplo para cobrir variações de "Easy Apply" e "Apply now".
	applyBtn := page.Locator("button:has-text('Easy Apply')").First()
	btnCount, err := applyBtn.Count()
	if err != nil || btnCount == 0 {
		return fmt.Errorf("botão de candidatura rápida não encontrado, possivelmente por mudança de layout ou candidatura já enviada")
	}

	if err := applyBtn.Click(); err != nil {
		return fmt.Errorf("falha ao clicar no botão de candidatura rápida: %w", err)
	}

	// Percorre as etapas do formulário até chegar ao envio final.
	pageLimit := 10 // Safe guard preventing infinite loops
	for i := 0; i < pageLimit; i++ {
		pw.HumanSleep()

		// Interrompe cedo se o modal de sucesso já apareceu.
		postApplyConf := page.Locator("text='Application sent'")
		if cnt, _ := postApplyConf.Count(); cnt > 0 {
			break
		}

		// Preenche os campos de texto do formulário com dados do perfil.
		if err := fillTextInputs(page, groqClient); err != nil {
			log.Printf("filler warning: failed mapping text inputs: %v", err)
		}

		// Anexa o currículo quando o formulário expõe um campo de arquivo.
		if err := handleFileUpload(page, c.Candidatura.CurriculoPath); err != nil {
			log.Printf("filler warning: failed attaching file: %v", err)
		}

		// Avança pelas etapas intermediárias até encontrar a ação final.
		nextBtn := page.Locator("button:has-text('Next')").First()
		reviewBtn := page.Locator("button:has-text('Review')").First()
		submitBtn := page.Locator("button:has-text('Submit application')").First()

		if ok, _ := submitBtn.IsVisible(); ok {
			// Envia a candidatura quando o botão final estiver disponível.
			if err := submitBtn.Click(); err != nil {
				return fmt.Errorf("falha ao enviar a candidatura: %w", err)
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
			// Sem botões padrão, a tela provavelmente está em um estado intermediário incomum.
			pw.HumanSleep()
		}
	}

	pw.HumanSleep()
	
	// Confirma que a tela final de sucesso realmente apareceu.
	successText := page.Locator("text='Application sent'")
	if cnt, _ := successText.Count(); cnt == 0 {
		return fmt.Errorf("não foi possível confirmar o estado final de sucesso da candidatura")
	}

	return nil
}

func fillTextInputs(page playwright.Page, groqClient *llm.GroqClient) error {
	// Localiza campos de texto comuns do formulário.
	inputs := page.Locator("input[type='text'], textarea, input[type='number'], input[type='tel']")
	count, err := inputs.Count()
	if err != nil {
		return err
	}

	for i := 0; i < count; i++ {
		inputLoc := inputs.Nth(i)
		
		// Ignora campos que já vieram preenchidos pela página.
		val, _ := inputLoc.InputValue()
		if val != "" {
			continue
		}

		// Extrai o texto do label associado ao campo.
		var labelText string
		id, _ := inputLoc.GetAttribute("id")
		if id != "" {
			labelLoc := page.Locator(fmt.Sprintf("label[for='%s']", id))
			if visible, _ := labelLoc.IsVisible(); visible {
				labelText, _ = labelLoc.InnerText()
			}
		}

		if labelText == "" {
			// Tenta obter o label implícito quando não há atributo for.
			labelLoc := page.Locator("label").Filter(playwright.LocatorFilterOptions{Has: inputLoc})
			if visible, _ := labelLoc.IsVisible(); visible {
				labelText, _ = labelLoc.InnerText()
			}
		}

		// Resolve a resposta com regra fixa ou com apoio do LLM.
		answer := getAnswerForLabel(labelText, groqClient)
		if answer != "" {
			// Digita com atraso humano para reduzir detecção de automação.
			if err := pw.TypeHumanly(inputLoc, answer); err != nil {
				log.Printf("TypeHumanly error: %v", err)
			}
		}
	}
	return nil
}

func handleFileUpload(page playwright.Page, resumePath string) error {
	// Localiza o campo de upload quando ele estiver visível.
	fileInput := page.Locator("input[type='file']").First()
	isVis, _ := fileInput.IsVisible()
	
	// O elemento pode estar oculto; por isso o arquivo é anexado diretamente.
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
	
	// Para campos complexos, delega a resposta ao Groq.
	profileContext := getEnv("USER_PROFILE_CONTEXT", "Backend developer experienced in Go, Rust, React. Lives in Brazil.")
	
	ans, err := groqClient.AnswerFormField(label, profileContext)
	if err != nil {
		log.Printf("groq warn: failed to answer question '%s': %v", label, err)
		return ""
	}
	return ans
}
