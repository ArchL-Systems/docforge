# DocForge

O DocForge transforma arquivos Markdown em um site estático.

## Gerar a documentação

```bash
go run ./cmd/docforge build --source docs --output dist
```

Abra `dist/index.html` no navegador. A navegação inclui os arquivos `.md`
encontrados recursivamente no diretório de origem.
