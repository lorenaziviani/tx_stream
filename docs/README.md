# Documentação do TxStream

Esta pasta contém toda a documentação do projeto TxStream, incluindo diagramas de arquitetura e fluxos do sistema.

## 📊 Diagramas Disponíveis

### 1. `architecture.drawio`

Diagrama da arquitetura geral do sistema TxStream, mostrando:

- Camadas da aplicação (API, Application, Domain, Infrastructure)
- Integração com PostgreSQL e Kafka
- Fluxo de dados e comunicação entre componentes
- Padrão Outbox implementado

### 2. `outbox-pattern.drawio`

Diagrama detalhado do padrão Outbox, ilustrando:

- Fluxo completo de uma transação
- Processo de criação de entidades e eventos
- Processamento assíncrono de eventos
- Benefícios do padrão implementado

## 🛠️ Como Visualizar os Diagramas

### Opção 1: Draw.io Online

1. Acesse [draw.io](https://app.diagrams.net/)
2. Clique em "Open Existing Diagram"
3. Selecione o arquivo `.drawio` desejado
4. O diagrama será carregado e você poderá editá-lo

### Opção 2: VS Code

1. Instale a extensão "Draw.io Integration" no VS Code
2. Abra qualquer arquivo `.drawio`
3. O diagrama será renderizado automaticamente
4. Você pode editar diretamente no VS Code

### Opção 3: GitHub

- Os diagramas são automaticamente renderizados no GitHub
- Basta navegar até o arquivo `.drawio` no repositório

## 📋 Estrutura dos Diagramas

### Arquitetura Geral

```
Cliente HTTP → API Gateway → Application Layer → Domain Layer
                                    ↓
                            Infrastructure Layer
                                    ↓
                    PostgreSQL ← → Kafka ← → Event Consumers
```

### Padrão Outbox

```
1. HTTP Request → 2. Begin Transaction → 3. Insert Order
                                            ↓
4. Insert Event → 5. Commit Transaction → 6. HTTP Response
                                            ↓
7. Background Process → 8. Publish to Kafka → 9. Mark as Published
```

## 🔄 Atualizando os Diagramas

Para manter a documentação atualizada:

1. **Edite os diagramas** usando draw.io
2. **Exporte como .drawio** para manter o formato editável
3. **Comite as mudanças** com uma mensagem descritiva
4. **Atualize esta documentação** se necessário

## 📝 Convenções

- Use cores consistentes para representar diferentes tipos de componentes
- Mantenha os diagramas simples e focados
- Inclua legendas quando necessário
- Use setas para mostrar o fluxo de dados
- Adicione comentários explicativos nos diagramas

## 🎯 Próximos Diagramas Planejados

- [ ] Diagrama de sequência para criação de pedidos
- [ ] Diagrama de deployment com Docker
- [ ] Diagrama de monitoramento e observabilidade
- [ ] Diagrama de testes e qualidade de código
