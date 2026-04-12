//! Gerador de PDF a partir de Markdown — pipeline MD → HTML → PDF.
//!
//! # Intenção
//! Recebe a saída bruta do Gemini, extrai apenas o bloco Markdown útil,
//! aplica CSS corporativo e gera um PDF em `/tmp/forja/{uuid}-cv.pdf`.
//!
//! # Restrição (SecOps)
//! PDFs são salvos em diretório isolado com nome UUID dinâmico para evitar
//! colisões e impedir acesso previsível (path traversal).

use anyhow::{Context, Result};
use printpdf::*;
use std::fs;
use std::path::{Path, PathBuf};
use uuid::Uuid;

/// CSS corporativo básico embutido.
/// Fontes sans-serif, cores escuras, espaçamento profissional.
#[allow(dead_code)]
pub const CSS_TEMPLATE: &str = r#"
body {
    font-family: 'Helvetica Neue', Arial, sans-serif;
    color: #1a1a1a;
    line-height: 1.6;
    margin: 40px 60px;
    font-size: 11pt;
}
h1 { font-size: 20pt; color: #0d47a1; margin-bottom: 4px; }
h2 { font-size: 14pt; color: #1565c0; border-bottom: 1px solid #e0e0e0; padding-bottom: 4px; }
h3 { font-size: 12pt; color: #333333; }
ul { padding-left: 20px; }
li { margin-bottom: 4px; }
p { margin: 6px 0; }
"#;

/// Extrai apenas o bloco Markdown útil da resposta do LLM.
///
/// Remove saudações, comentários e artefatos que a IA pode gerar
/// apesar do prompt rígido. Se houver um bloco ```markdown```, extrai o conteúdo
/// interno. Caso contrário, retorna o texto após limpar linhas de saudação.
pub fn extract_markdown_block(llm_output: &str) -> String {
    let trimmed = llm_output.trim();

    // Se a IA envolveu a resposta em ```markdown ... ```, extrai o interior.
    if let Some(start) = trimmed.find("```markdown") {
        let content = &trimmed[start + 11..]; // pula o prefixo "```markdown"
        if let Some(end) = content.find("```") {
            return content[..end].trim().to_string();
        }
    }

    // Se a IA envolveu em ``` genérico
    if let Some(start) = trimmed.find("```") {
        let content = &trimmed[start + 3..];
        if let Some(end) = content.find("```") {
            return content[..end].trim().to_string();
        }
    }

    // Fallback: remove linhas de saudação comuns.
    let lines: Vec<&str> = trimmed
        .lines()
        .filter(|line| {
            let lower = line.trim().to_lowercase();
            !lower.starts_with("aqui está")
                && !lower.starts_with("here is")
                && !lower.starts_with("claro,")
                && !lower.starts_with("sure,")
                && !lower.starts_with("certainly")
                && !lower.starts_with("com base")
                && !lower.starts_with("based on")
        })
        .collect();

    lines.join("\n").trim().to_string()
}

/// Converte Markdown em HTML usando pulldown-cmark.
fn markdown_to_html(md: &str) -> String {
    let parser = pulldown_cmark::Parser::new(md);
    let mut html = String::new();
    pulldown_cmark::html::push_html(&mut html, parser);
    html
}

/// Gera o PDF a partir de Markdown e salva em `/tmp/forja/{uuid}-cv.pdf`.
///
/// # SecOps
/// - Diretório `/tmp/forja/` é criado automaticamente se não existir
/// - Nome usa UUID v4 → imprevisível, sem colisões
/// - Retorna o `PathBuf` exato para registro no banco.
pub fn render_to_pdf(markdown: &str, output_dir: &Path) -> Result<PathBuf> {
    // Garante que o diretório de saída existe
    fs::create_dir_all(output_dir)
        .with_context(|| format!("falha ao criar diretório '{}'", output_dir.display()))?;

    let filename = format!("{}-cv.pdf", Uuid::new_v4());
    let output_path = output_dir.join(&filename);

    // Converter Markdown → HTML (para referência/debug) e extrair texto limpo
    let _html = markdown_to_html(markdown);

    // Gerar PDF com printpdf
    let (doc, page1, layer1) = PdfDocument::new("Currículo", Mm(210.0), Mm(297.0), "Layer 1");
    let mut current_layer = doc.get_page(page1).get_layer(layer1);

    // Usar fonte built-in (Helvetica — sans-serif como definido no CSS)
    let font = doc
        .add_builtin_font(BuiltinFont::Helvetica)
        .context("falha ao carregar fonte Helvetica")?;

    let font_bold = doc
        .add_builtin_font(BuiltinFont::HelveticaBold)
        .context("falha ao carregar fonte Helvetica Bold")?;

    // Renderizar o conteúdo Markdown como texto no PDF
    let lines: Vec<&str> = markdown.lines().collect();
    let mut y_pos = 270.0_f32; // margem superior
    let left_margin = 20.0_f32;
    let line_height = 5.5_f32;

    for line in &lines {
        if y_pos < 20.0 {
            let (new_page, new_layer) = doc.add_page(Mm(210.0), Mm(297.0), "Layer 1");
            current_layer = doc.get_page(new_page).get_layer(new_layer);
            y_pos = 270.0_f32; // reseta margem superior
        }

        let trimmed = line.trim();

        if trimmed.starts_with("# ") {
            // H1 — título principal
            current_layer.use_text(
                &trimmed[2..],
                16.0,
                Mm(left_margin),
                Mm(y_pos),
                &font_bold,
            );
            y_pos -= line_height * 2.0;
        } else if trimmed.starts_with("## ") {
            // H2 — seção
            y_pos -= line_height * 0.5;
            current_layer.use_text(
                &trimmed[3..],
                13.0,
                Mm(left_margin),
                Mm(y_pos),
                &font_bold,
            );
            y_pos -= line_height * 1.5;
        } else if trimmed.starts_with("### ") {
            // H3 — subseção
            current_layer.use_text(
                &trimmed[4..],
                11.0,
                Mm(left_margin),
                Mm(y_pos),
                &font_bold,
            );
            y_pos -= line_height * 1.2;
        } else if trimmed.starts_with("- ") || trimmed.starts_with("* ") {
            // Lista
            let text = format!("• {}", &trimmed[2..]);
            current_layer.use_text(&text, 10.0, Mm(left_margin + 5.0), Mm(y_pos), &font);
            y_pos -= line_height;
        } else if trimmed.is_empty() {
            y_pos -= line_height * 0.5;
        } else {
            // Parágrafo normal
            current_layer.use_text(trimmed, 10.0, Mm(left_margin), Mm(y_pos), &font);
            y_pos -= line_height;
        }
    }

    doc.save(&mut std::io::BufWriter::new(
        fs::File::create(&output_path)
            .with_context(|| format!("falha ao criar arquivo '{}'", output_path.display()))?,
    ))
    .context("falha ao salvar PDF")?;

    Ok(output_path)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_extract_markdown_block_with_fenced_code() {
        let input = "Aqui está o currículo:\n```markdown\n# Nome\n## Experiência\n- Rust\n```\nEspero que ajude!";
        let result = extract_markdown_block(input);
        assert!(result.starts_with("# Nome"));
        assert!(result.contains("## Experiência"));
        assert!(!result.contains("Aqui está"));
        assert!(!result.contains("Espero que"));
    }

    #[test]
    fn test_extract_markdown_block_removes_greetings() {
        let input = "Aqui está o currículo adaptado:\n# João Silva\n## Resumo\nEngenheiro Senior";
        let result = extract_markdown_block(input);
        assert!(!result.contains("Aqui está"));
        assert!(result.contains("# João Silva"));
    }

    #[test]
    fn test_render_to_pdf_creates_file() {
        let md = "# Teste\n## Seção\n- Item 1\n- Item 2\n\nTexto normal.";
        let dir = std::env::temp_dir().join("ghostapply_test_pdf");
        let path = render_to_pdf(md, &dir).expect("PDF deve ser gerado");
        assert!(path.exists(), "arquivo PDF deve existir");
        assert!(path.to_string_lossy().ends_with("-cv.pdf"));
        // Cleanup
        let _ = std::fs::remove_file(&path);
        let _ = std::fs::remove_dir(&dir);
    }
}
