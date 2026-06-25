# Video Converter

Conversor de vídeos em lote com `ffmpeg` que **escolhe sozinho o melhor encoder da sua máquina**
(GPU NVIDIA/Intel/AMD/Apple ou CPU) e converte todos os vídeos de uma pasta. Por padrão reduz para
**30 fps**, redimensiona para **1080p** e **copia o áudio sem reencodar** — tudo configurável pelo `.env`.

## Como executar (3 passos)

Pré-requisito: **ffmpeg** instalado e no `PATH` (`ffmpeg -version` para conferir) e **Go 1.22+**.

> Linux/Debian: `sudo apt install ffmpeg` · macOS: `brew install ffmpeg` · Windows: `winget install Gyan.FFmpeg`

```bash
# 1. Configure (copie o exemplo e ajuste INPUT_DIR / OUTPUT_DIR no .env)
cp .env.example .env

# 2. Compile
go build -o video-converter .

# 3. Rode (lê o .env da pasta atual)
./video-converter
```

Pronto. Ele detecta o encoder, mostra o que escolheu e converte tudo que encontrar em `INPUT_DIR`
(incluindo subpastas). A saída vai para `<OUTPUT_DIR>/<nome-da-pasta-de-entrada>_CONV/`, espelhando a
estrutura, em `.mp4`. Rodar de novo **pula** o que já foi convertido; `Ctrl-C` encerra sem deixar
arquivos corrompidos.

## Configuração (`.env`)

Edite o `.env`. No mínimo, defina `INPUT_DIR` e `OUTPUT_DIR`. Toda opção tem um padrão sensato.

| Variável | Padrão | Descrição |
|---|---|---|
| `INPUT_DIR` | `./in` | pasta com os vídeos de entrada (varre subpastas) |
| `OUTPUT_DIR` | `./out` | pasta base de saída (use uma pasta diferente da entrada) |
| `BACKEND` | `auto` | `auto`/`cpu`/`nvenc`/`qsv`/`amf`/`vaapi`/`videotoolbox` |
| `CODEC` | `h264` | `h264`/`hevc`/`av1` (`hevc` gera arquivos menores) |
| `QUALITY` | `60` | 0..100 (maior = melhor qualidade, arquivo maior) |
| `FPS` | `30` | limita o frame rate (ex.: `20`); `0` = manter o original |
| `SCALE` | `1080` | limita a altura em px (ex.: `720`); `0` = manter o tamanho original |
| `INTENSITY` | `balanced` | `light`/`balanced`/`max` (quanto puxa da máquina) |
| `OVERWRITE` | `false` | `true` refaz mesmo o que já foi convertido |

> O áudio é sempre **copiado** (`-c:a copy`) — rápido e sem perda, sem reencode.
>
> `FPS` e `SCALE` nunca aumentam: um vídeo a 24 fps / 720p não é "esticado" para 30 fps / 1080p.

Opções avançadas (workers, threads, extensões, fallback, vaapi-device, nice, log-dir) estão comentadas
no `.env.example`. Qualquer variável pode ser sobrescrita por uma flag de mesmo nome
(ex.: `./video-converter -fps 20 -scale 720 -backend cpu`); a precedência é **flag > `.env` > padrão**.

## Verificação

```bash
go test ./...        # unitários + integração (gera um clipe com ffmpeg e converte na CPU)
go test -short ./... # só unitários (sem ffmpeg)
```

## Como funciona

```
config (.env + flags) → detecção + probe funcional → varredura de arquivos
   → pool de workers (errgroup + context) → relatório
```

Antes de usar um encoder, o programa faz um *probe* real (codifica 1 frame). Se a GPU/driver não
funcionar de verdade, ele detecta e cai para o próximo backend — terminando sempre na CPU
(libx264/x265/SVT-AV1). Adicionar um novo encoder = 1 arquivo implementando `EncoderBackend` + 1 entrada
no registry.
