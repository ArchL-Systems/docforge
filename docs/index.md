# DocForge

O DocForge transforma arquivos Markdown em um site estático.

## Gerar a documentação

```bash
go run ./cmd/docforge build --output dist
```

Rode na raiz do projeto: o `DOCS.md` lista as pastas lidas. Abra
`dist/docs/index.html` no navegador. A navegação inclui os arquivos `.md`
encontrados recursivamente nessas pastas.
