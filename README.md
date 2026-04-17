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

### Конфиг-файл

Файл `clangd-mcp.cfg` в формате JSON, расположенный рядом с исполняемым файлом:

```json
{
  "clangd_path": "C:/Program Files/LLVM/bin/clangd.exe",
  "port": 7878
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
| `workspace_symbol` | `workspace/symbol` | Поиск символов по имени |
| `workspace_symbolResolve` | `workspace/symbolResolve` | Получить полную локацию WorkspaceSymbol |
| `textDocument_references` | `textDocument/references` | Все места использования символа |
| `textDocument_rename` | `textDocument/rename` | Переименование символа по всему проекту |
| `textDocument_hover` | `textDocument/hover` | Тип и документация символа |
| `textDocument_declaration` | `textDocument/declaration` | Перейти к объявлению |
| `textDocument_definition` | `textDocument/definition` | Перейти к определению |
| `textDocument_typeDefinition` | `textDocument/typeDefinition` | Перейти к определению типа |
| `textDocument_implementation` | `textDocument/implementation` | Найти реализации |
| `textDocument_prepareCallHierarchy` | `textDocument/prepareCallHierarchy` | Подготовить узлы иерархии вызовов |
| `callHierarchy_incomingCalls` | `callHierarchy/incomingCalls` | Кто вызывает функцию |
| `callHierarchy_outgoingCalls` | `callHierarchy/outgoingCalls` | Что вызывает функция |
| `textDocument_documentSymbol` | `textDocument/documentSymbol` | Все символы в файле |
| `textDocument_prepareTypeHierarchy` | `textDocument/prepareTypeHierarchy` | Подготовить узлы иерархии типов |
| `typeHierarchy_supertypes` | `typeHierarchy/supertypes` | Найти суперклассы и интерфейсы |
| `typeHierarchy_subtypes` | `typeHierarchy/subtypes` | Найти подклассы и реализации |

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
