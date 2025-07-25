#!/bin/bash

# Script para alternar usuários Git
# Uso: ./git-switch.sh [pessoal|trabalho|lista]

case "$1" in
    "pessoal"|"personal")
        echo "🏠 Configurando Git para uso PESSOAL..."
        git config user.name "lorenaziviani"
        git config user.email "lorena.ziviani.andrade@gmail.com"
        echo "✅ Configurado como: $(git config user.name) <$(git config user.email)>"
        ;;
    
    "trabalho"|"work")
        echo "🏢 Configurando Git para uso TRABALHO..."
        git config user.name "Lorena Ziviani Andrade"
        git config user.email "lorena.andrade@track.co"
        echo "✅ Configurado como: $(git config user.name) <$(git config user.email)>"
        ;;
    
    "lista"|"list"|"status")
        echo "📋 Configuração atual:"
        echo "Nome: $(git config user.name)"
        echo "Email: $(git config user.email)"
        echo "Remote: $(git remote get-url origin 2>/dev/null || echo 'Nenhum remote configurado')"
        ;;
    
    *)
        echo "🔀 Git User Switcher"
        echo ""
        echo "Uso: $0 [opção]"
        echo ""
        echo "Opções:"
        echo "  pessoal    - Configurar para uso pessoal (lorenaziviani)"
        echo "  trabalho   - Configurar para uso trabalho (lorenazivianiandrade)"
        echo "  lista      - Mostrar configuração atual"
        echo ""
        echo "Configuração atual:"
        echo "  Nome: $(git config user.name 2>/dev/null || echo 'Não configurado')"
        echo "  Email: $(git config user.email 2>/dev/null || echo 'Não configurado')"
        ;;
esac 

# git remote set-url origin git@github.com-personal:lorenaziviani/tx_stream.git 