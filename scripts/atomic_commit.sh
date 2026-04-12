#!/bin/bash
# Script para realizar commits atômicos por arquivo seguindo Conventional Commits

# Lista arquivos alterados
files=$(git status --porcelain | awk '{print $2}')

if [ -z "$files" ]; then
    echo "No changes to commit."
    exit 0
fi

for file in $files; do
    echo "Processing $file..."
    
    # Determina o escopo baseado na pasta
    scope=$(dirname "$file" | cut -d'/' -f1,2)
    filename=$(basename "$file")
    
    # Pergunta a mensagem para o arquivo ou gera uma automática (IA pode preencher aqui)
    # No caso de automação total pela IA, ela chamaria o git add e git commit diretamente.
    # Este script serve como facilitador.
    
    git add "$file"
    
    # Exemplo de mensagem curta e técnica
    msg="chore($scope): sync update for $filename"
    
    git commit -m "$msg"
done
