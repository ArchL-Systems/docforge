# DocForge

> Um compilador de documentação para repositórios: lê o projeto, entende as instruções do desenvolvedor e transforma código e Markdown em documentação completa e navegável.

## O que é

DocForge é uma ferramenta CLI, open-source, que transforma um repositório de código em um site de documentação estático — navegável, com busca, temas claro/escuro e diagramas.

Diferente de gerar documentação "pedindo pra uma IA escrever", o DocForge propõe uma camada de engenharia entre o repositório e a IA:

```
Código + conhecimento existente + instruções do desenvolvedor → documentação estruturada
```

O desenvolvedor define o que deve ser documentado através de um arquivo `DOCS.md` na raiz do projeto. O DocForge usa essas instruções, os arquivos `.md` espalhados pelo repositório, a estrutura de pastas e, opcionalmente, o próprio código-fonte para construir a documentação.

## Status

🚧 **Em desenvolvimento inicial.** Projeto pessoal/playground, sem cronograma fixo.

## Como funciona (visão geral)

```
Repository
    │
    ▼
Scanner (markdown, source, estrutura, OpenAPI, git metadata)
    │
    ▼
Context Builder
    │
    ▼
Documentation Engine (geração determinística + IA opcional)
    │
    ▼
Documentation Renderer
    │
    ▼
Static Documentation
```

Um exemplo de `DOCS.md`:

```markdown
# Documentation Instructions

## Architecture
- Explain the project architecture
- Identify the main layers
- Generate an architecture diagram

## Modules
- Scan src/modules
- Use README.md files as additional context
- Document the responsibility of each module

## API
- Read the OpenAPI specification
- Document available endpoints

## Output
- Generate a searchable static website
```

## Roadmap

- **v1** — CLI + scanner determinístico + parser do `DOCS.md` + renderer estático (navegação, busca, tema claro/escuro)
- **v2** — `docforge dev` com hot reload, diagramas automáticos, cross-references, suporte a OpenAPI
- **v3** — camada de IA opcional (via integração com agente / MCP, com Ollama como opção local)

## Stack

Escrito em [Go](https://go.dev/), com Goldmark para Markdown e
`html/template` + `embed` para gerar páginas com os assets incluídos no executável.

## Desenvolvimento

Pré-requisito: Go 1.26 ou superior.

```bash
go mod download
go test ./...
go build -o bin/docforge ./cmd/docforge
go run ./cmd/docforge --help
```

Qualidade de código: `golangci-lint fmt` e `golangci-lint run ./...`.
Veja [CONTRIBUTING.md](CONTRIBUTING.md) para instalar a versão usada na CI.

## Uso atual

```bash
go run ./cmd/docforge build --source docs --output dist
```

Abra `dist/index.html` no navegador para ver a documentação de exemplo.
Os arquivos `.md` são encontrados recursivamente e convertidos em `.html`,
preservando os diretórios. Cada página inclui navegação para as demais.
Para ter uma página inicial, inclua `index.md` na origem.

Origem e destino precisam ser diretórios separados, sem sobreposição — o
build limpa o diretório de saída antes de gerar, então ele que gerencia o
conteúdo do destino. Links simbólicos na origem são ignorados. Links Markdown
relativos a páginas conhecidas são reescritos para `.html` (mantendo âncoras);
links externos e para arquivos desconhecidos permanecem como estão. Arquivos
que não são Markdown (imagens, CSS e afins) são copiados para a saída
preservando a estrutura de pastas.

O parser de `DOCS.md`, análise de TypeScript, busca, IA e o servidor de
desenvolvimento ainda fazem parte do roadmap.

## Uso planejado (ainda não disponível)

```bash
npx docforge init   # cria as configurações iniciais
npx docforge build  # analisa o repositório e gera a documentação
npx docforge dev    # servidor local com atualização automática
```

## Licença

[MIT](LICENSE)
