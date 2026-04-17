# clangd-mcp

Прозрачный LSP-прокси для clangd со встроенным MCP/SSE сервером. Позволяет AI-агентам использовать возможности clangd (поиск символов, поиск ссылок, переименование) через ту же конфигурацию, что и ваша IDE.

## Архитектура

```
IDE (QtCreator) ──stdio──► clangd-mcp ──stdio──► clangd
                                │
                           HTTP/SSE :port
                                │
                        AI Agent (Claude, etc.)
```

clangd-mcp встаёт между IDE и clangd, прозрачно проксируя весь LSP-трафик. Параллельно поднимается SSE-сервер, через который AI-агенты могут вызывать MCP-тулы, использующие тот же экземпляр clangd.

### Мультиплексирование запросов

Прокси различает источники запросов через префиксы ID:
- `Q<n>` — запросы от IDE, ответы возвращаются в stdout
- `M<n>` — запросы от MCP-тулов, ответы возвращаются через канал

## Сборка

```bash
go build -o clangd-mcp.exe .
```

## Настройка IDE

### QtCreator

Preferences → C++ → Clang Code Model → Override clangd executable → укажите путь до `clangd-mcp.exe`.

Дополнительных аргументов не требуется — прокси прозрачно передаёт все аргументы в clangd.

**Важно:** MCP-сервер запускается только при наличии флага `--compile-commands-dir` в аргументах clangd.

## Конфигурация

Конфигурация определяется с приоритетом: переменные окружения > конфиг-файл > значения по умолчанию.

### Переменные окружения

| Переменная | Описание | По умолчанию |
|---|---|---|
| `CLANGD_MCP_CLANGD_PATH` | Путь до бинарника clangd | Поиск в PATH |
| `CLANGD_MCP_PORT` | Порт MCP-сервера | `7878` |
| `CLANGD_MCP_DEBUG_SSE` | Включить детальное логирование SSE запросов/ответов | `0` |

### Конфиг-файл

Файл `clangd-mcp.cfg` в формате JSON, расположенный рядом с исполняемым файлом:

```json
{
  "clangd_path": "C:/Program Files/LLVM/bin/clangd.exe",
  "port": 7878,
  "debug_sse": false
}
```

### Порядок поиска clangd

1. Переменная окружения `CLANGD_MCP_CLANGD_PATH`
2. Поле `clangd_path` из конфиг-файла
3. Поиск `clangd` в системном PATH

## MCP-тулы

Имена тулов повторяют названия LSP-методов (`/` заменяется на `_`).

| Тул | LSP-метод | Описание |
|---|---|---|
| `callHierarchy_incomingCalls` | `callHierarchy/incomingCalls` | Кто вызывает функцию |
| `callHierarchy_outgoingCalls` | `callHierarchy/outgoingCalls` | Что вызывает функция |
| `textDocument_declaration` | `textDocument/declaration` | Перейти к объявлению |
| `textDocument_definition` | `textDocument/definition` | Перейти к определению |
| `textDocument_documentSymbol` | `textDocument/documentSymbol` | Все символы в файле |
| `textDocument_hover` | `textDocument/hover` | Тип и документация символа |
| `textDocument_implementation` | `textDocument/implementation` | Найти реализации |
| `textDocument_prepareCallHierarchy` | `textDocument/prepareCallHierarchy` | Подготовить узлы иерархии вызовов |
| `textDocument_prepareTypeHierarchy` | `textDocument/prepareTypeHierarchy` | Подготовить узлы иерархии типов |
| `textDocument_references` | `textDocument/references` | Все места использования символа |
| `textDocument_rename` | `textDocument/rename` | Переименование символа по всему проекту |
| `textDocument_typeDefinition` | `textDocument/typeDefinition` | Перейти к определению типа |
| `typeHierarchy_subtypes` | `typeHierarchy/subtypes` | Найти подклассы и реализации |
| `typeHierarchy_supertypes` | `typeHierarchy/supertypes` | Найти суперклассы и интерфейсы |
| `workspace_symbol` | `workspace/symbol` | Поиск символов по имени |
| `workspace_symbolResolve` | `workspace/symbolResolve` | Получить полную локацию WorkspaceSymbol |

## Подключение MCP-клиента

```json
{
  "mcpServers": {
    "clangd": {
      "type": "sse",
      "url": "http://localhost:7878/sse"
    }
  }
}
```

Порт в URL должен совпадать с настроенным портом (по умолчанию 7878).

## Логирование

Лог-файл `clangd-mcp.log` создаётся в той же директории, что и исполняемый файл.

### Базовое логирование

По умолчанию логируются:
- Запуск/остановка приложения
- Загруженная конфигурация
- Ошибки подключения и таймауты
- Базовые события жизненного цикла

### Детальное SSE логирование

Для отладки MCP-инструментов можно включить детальное логирование всех SSE запросов и ответов.

**Через переменную окружения:**
```bash
set CLANGD_MCP_DEBUG_SSE=1
clangd-mcp.exe
```

**Через конфиг-файл:**
```json
{
  "debug_sse": true
}
```

**Что логируется при включении `debug_sse`:**
- Входящие MCP-запросы с аргументами (JSON-форматированные)
- Исходящие MCP-ответы (результаты выполнения инструментов)
- LSP-запросы, отправляемые MCP-инструментами
- LSP-ответы с результатами (большие результаты обрезаются)
- Ошибки, возникшие при выполнении инструментов

Все логи с SSE информацией помечены префиксом `[SSE-DEBUG]` для удобной фильтрации.
