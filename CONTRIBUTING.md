# Contribuindo com o DocForge

Obrigado pelo interesse em contribuir! O projeto ainda está em fase inicial, então as regras abaixo são propositalmente enxutas.

## Antes de começar

- Abra uma **issue** antes de uma PR grande, pra alinhar escopo e evitar retrabalho.
- Para bugs simples, typos ou melhorias pequenas, pode ir direto pra PR.

## Ambiente de desenvolvimento

Pré-requisitos:

- [Go](https://go.dev/dl/) 1.26 ou superior
- [golangci-lint](https://golangci-lint.run/docs/welcome/install/local/) v2.15.0 (mesma versão da CI)

```bash
git clone https://github.com/gabbzin/docforge.git
cd docforge
go mod download
go build ./cmd/docforge
go test ./...
```

## Padrões de código

- Rode `golangci-lint fmt` antes de commitar (gofmt e goimports).
- Rode `golangci-lint config verify` e `golangci-lint run ./...` antes de abrir a PR.
- A configuração `.golangci.yml` inclui os linters padrão, como govet, staticcheck e errcheck, além de errorlint, misspell, nolintlint e unconvert.
- Mantenha commits pequenos e com mensagens claras (preferencialmente no formato [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `docs:`, `chore:`, etc.).

## Pull Requests

1. Fork o repositório e crie uma branch a partir de `main`: `git checkout -b feat/minha-feature`.
2. Garanta que `go build ./cmd/docforge`, `go test -race -shuffle=on ./...` e `golangci-lint run ./...` passam localmente.
3. Descreva na PR o que mudou e por quê.
4. Uma PR só é mergeada após revisão e aprovação.

## Integração contínua

A CI executa lint e formatação, verifica os módulos com `go mod verify` e
`go mod tidy -diff`, e roda testes com detecção de race e cobertura em Linux,
macOS e Windows. Cada plataforma também compila a CLI e gera a documentação
de exemplo. A versão de Go vem do `go.mod`.

As actions são fixadas por SHA. O Dependabot verifica semanalmente atualizações
das actions e dependências Go. Ao atualizar o golangci-lint na CI, atualize também
a versão indicada neste guia.

## Código de conduta

Ao contribuir, você concorda em seguir o [Código de Conduta](CONDUCT.md) do projeto.

## Dúvidas

Abra uma issue com a tag `question`.
