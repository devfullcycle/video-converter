
# Video Converter



### Executar localmente

Clone o projeto

```bash
  git clone https://github.com/devfullcycle/video-converter.git
```

Navegue até a pasta do projeto

```bash
  cd video-converter
```

Instale as dependencias de acordo com seu S.O


#### MAC:
```bash
  brew install ffmpeg
```
#### WINDOWS:
https://www.ffmpeg.org/download.html

#### LINUX
```bash
  sudo apt install ffmpeg 
```

Execute o comando:

```bash
  go run main.go -input <INPUT_DIR> -output <OUTPUT_DIR>
```

- INPUT_DIR: Local onde os arquivos que precisam de conversão se encontram
- OUTPUT_DIR: Local onde os arquivos convertidos serão criados
