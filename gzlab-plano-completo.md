# GitLab TUI — Plano Completo do Produto

> Status: planejamento inicial  
> Objetivo: construir um TUI para GitLab focado em Merge Requests, diffs, pipelines e fluxo de trabalho multi-profile.

---

## 1. Visão do Produto

A ideia é criar uma interface terminal para GitLab, inspirada em ferramentas como `lazygit`, `k9s`, `gh` e outros TUIs produtivos.

O foco inicial não é recriar o GitLab inteiro, mas resolver bem o fluxo diário de desenvolvimento:

- navegar entre projetos;
- abrir branches recentes;
- visualizar Merge Requests;
- revisar diffs com conforto;
- acompanhar pipelines;
- abrir logs de jobs;
- operar em GitLab empresarial e pessoal via profiles;
- reduzir dependência do navegador para tarefas comuns.

---

## 2. Decisões Já Definidas

### 2.1 Autenticação

A autenticação inicial será feita exclusivamente via token.

Por enquanto, não entra OAuth.

O token será usado para consumir a API do GitLab.

Deve haver suporte a múltiplos profiles, por exemplo:

- GitLab empresarial;
- GitLab pessoal;
- outros GitLabs self-hosted.

Exemplo conceitual:

```yaml
default_profile: empresa

profiles:
  empresa:
    host: https://gitlab.empresa.com
    token_env: GITLAB_EMPRESA_TOKEN

  pessoal:
    host: https://gitlab.com
    token_env: GITLAB_PESSOAL_TOKEN
```

### 2.2 GitLab como backend único

Toda a lógica inicial será baseada em GitLab.

Não haverá suporte a GitHub, Bitbucket ou outros provedores no começo.

### 2.3 Issues fora do MVP inicial

Issues devem ficar registradas como melhoria futura.

Não fazem parte dos MVPs iniciais.

### 2.4 Foco principal

Os principais módulos iniciais serão:

- profiles;
- projetos recentes;
- branches recentes;
- Merge Requests;
- diffs;
- pipelines;
- logs de jobs;
- busca;
- histórico de navegação;
- configuração de layout.

---

## 3. Conceito de Experiência

O usuário abre o TUI e consegue rapidamente responder perguntas como:

- Quais MRs precisam da minha atenção?
- Qual era aquela branch recente em que eu estava trabalhando?
- Essa MR está com pipeline verde?
- O que mudou nesse diff?
- Qual job falhou?
- Onde está o erro no log?
- Qual projeto eu acessei recentemente?
- Consigo trocar entre GitLab empresa e pessoal?

---

## 4. Estrutura Principal do App

```text
GitLab TUI
├── Profiles
├── Dashboard
├── Recent Projects
├── Recent Branches
├── Projects
├── Merge Requests
│   ├── Overview
│   ├── Diff
│   ├── Discussions
│   ├── Commits
│   └── Pipeline
├── Pipelines
│   ├── Jobs
│   └── Logs
├── Search
├── Config
└── Future
    └── Issues
```

---

# 5. MVPs

## MVP 1 — Foundation + GitLab Profiles + MR Cockpit

Objetivo: permitir autenticação por token, múltiplos profiles, detecção de projeto GitLab e visualização básica de MRs.

### Escopo

- autenticação via token;
- profiles empresarial/pessoal;
- configuração local;
- dashboard inicial;
- últimos projetos acessados;
- branches recentes;
- listagem de Merge Requests;
- detalhe básico de MR;
- abrir MR no navegador;
- copiar link da MR;
- visualizar status de pipeline da MR.

### Fora do escopo

- issues;
- comentários inline;
- resolver threads;
- merge pela ferramenta;
- review completo;
- criação de MR;
- OAuth.

---

## MVP 2 — Diff Viewer Forte

Objetivo: tornar o TUI realmente útil para revisar código.

### Escopo

- listar arquivos alterados da MR;
- abrir diff de cada arquivo;
- suporte a visualização horizontal lado a lado;
- suporte a visualização unificada com `+` e `-`;
- alternar layout por configuração;
- navegar entre arquivos;
- navegar entre hunks;
- buscar dentro do diff;
- abrir arquivo no editor externo;
- abrir arquivo no navegador;
- destacar adições e remoções;
- esconder ou mostrar whitespace.

### Decisão importante

A visualização horizontal deve ser prioridade porque parece mais confortável para comparar alterações.

Mas o usuário deve poder configurar:

```yaml
diff:
  mode: side_by_side
```

ou:

```yaml
diff:
  mode: unified
```

---

## MVP 3 — Pipelines + Logs de Jobs

Objetivo: permitir acompanhar CI/CD direto do terminal.

### Escopo

- ver pipeline associada à MR;
- listar jobs da pipeline;
- ver status por stage;
- abrir logs de job;
- buscar no log;
- pular para erro provável;
- destacar mensagens comuns de erro;
- retry de job;
- retry de pipeline;
- cancelar pipeline;
- rodar job manual, se permitido.

### Logs

O log viewer deve ter foco em produtividade.

Funcionalidades:

- busca textual;
- próximo resultado;
- voltar resultado;
- pular para `ERROR`;
- pular para `FAILED`;
- pular para `Exception`;
- pular para `BUILD FAILURE`;
- copiar trecho;
- salvar log em arquivo;
- abrir log no editor.

---

## MVP 4 — Workspace Multi-Repo + Resumo Copiável

Objetivo: ajudar em tarefas que envolvem múltiplos repositórios.

### Escopo

- criar workspace por tarefa;
- associar MRs ao workspace;
- agrupar por prefixo de branch, por exemplo `feature-PD-26527`;
- mostrar status geral de todas as MRs;
- mostrar pipelines de todas as MRs;
- mostrar approvals;
- mostrar threads pendentes;
- gerar resumo copiável para Google Chat, Slack ou Markdown.

Exemplo:

```text
Estado do PD-26527

✅ protocolo-model-commons !1475 — pipeline passou
✅ api-protocolo-cadastros-dados !91 — aprovado
🟡 api-protocolo-cadastros !250 — pipeline rodando
```

---

## MVP 5 — Review Actions

Objetivo: permitir ações reais de review dentro do TUI.

### Escopo

- aprovar MR;
- remover aprovação;
- comentar MR;
- responder discussão;
- resolver thread;
- reabrir thread;
- marcar arquivo como visto;
- checkout da branch da MR;
- atualizar branch local;
- abrir editor externo para comentários longos.

---

## MVP 6 — Criação e Edição de MR

Objetivo: permitir criar e editar MRs sem sair do terminal.

### Escopo

- criar MR da branch atual;
- detectar target branch;
- sugerir título baseado no nome da branch;
- escolher reviewers;
- escolher assignees;
- escolher labels;
- aplicar template de MR;
- editar descrição;
- marcar como draft;
- marcar como ready;
- copiar link final.

---

## MVP 7 — Issues e Boards

Objetivo: adicionar gestão de issues depois que o fluxo de MR estiver maduro.

### Escopo futuro

- listar issues atribuídas ao usuário;
- buscar issues;
- filtrar por label;
- filtrar por milestone;
- abrir issue;
- comentar issue;
- fechar issue;
- reabrir issue;
- criar issue;
- board Kanban simples.

---

# 6. Backlog Detalhado por Épicos

---

## Épico 1 — Setup do Projeto

### Objetivo

Criar a base técnica do projeto.

### Tasks

- [ ] Escolher linguagem principal.
- [ ] Criar repositório.
- [ ] Definir nome do binário.
- [ ] Criar estrutura inicial de pastas.
- [ ] Configurar build local.
- [ ] Configurar lint.
- [ ] Configurar testes.
- [ ] Criar README inicial.
- [ ] Criar licença.
- [ ] Definir padrão de commits.
- [ ] Definir versionamento.
- [ ] Criar primeiro comando `help`.
- [ ] Criar primeiro comando `version`.

### Sugestão de stack

Recomendado:

```text
Go + Bubble Tea + Lip Gloss + Bubbles + go-gitlab
```

Motivos:

- binário único;
- fácil distribuir;
- ótimo ecossistema de TUI;
- performance boa;
- aparência profissional;
- parecido com ferramentas como `lazygit`.

---

## Épico 2 — Configuração Local

### Objetivo

Permitir que o usuário configure o TUI localmente.

### Tasks

- [ ] Definir caminho do arquivo de configuração.
- [ ] Criar parser de YAML.
- [ ] Criar modelo de config.
- [ ] Criar config default.
- [ ] Implementar leitura de config.
- [ ] Implementar escrita de config.
- [ ] Validar config.
- [ ] Criar comando para mostrar config atual.
- [ ] Criar comando para editar config.
- [ ] Suportar variáveis de ambiente.
- [ ] Suportar expansão de `~`.
- [ ] Criar mensagens amigáveis para config inválida.

### Caminho sugerido

```text
~/.config/gitlab-tui/config.yaml
```

### Exemplo

```yaml
default_profile: empresa

profiles:
  empresa:
    host: https://gitlab.empresa.com
    token_env: GITLAB_EMPRESA_TOKEN

  pessoal:
    host: https://gitlab.com
    token_env: GITLAB_PESSOAL_TOKEN

ui:
  theme: dark
  mouse: true
  editor: "code --wait"
  browser: "default"

diff:
  mode: side_by_side
  ignore_whitespace: false

cache:
  enabled: true
```

---

## Épico 3 — Autenticação por Token

### Objetivo

Permitir autenticação segura no GitLab usando token.

### Tasks

- [ ] Criar comando `auth login`.
- [ ] Perguntar host do GitLab.
- [ ] Perguntar nome do profile.
- [ ] Perguntar token de forma oculta.
- [ ] Validar token chamando endpoint de usuário atual.
- [ ] Salvar profile.
- [ ] Permitir token via variável de ambiente.
- [ ] Permitir token direto no config apenas como fallback.
- [ ] Criar comando `auth status`.
- [ ] Criar comando `auth logout`.
- [ ] Mostrar usuário autenticado.
- [ ] Tratar token inválido.
- [ ] Tratar host inválido.
- [ ] Tratar GitLab indisponível.
- [ ] Tratar ausência de escopos necessários.

### Segurança

- [ ] Não imprimir token no terminal.
- [ ] Não salvar token em logs.
- [ ] Mascarar token em debug.
- [ ] Preferir variável de ambiente ou keychain no futuro.
- [ ] Documentar escopos necessários.

### Escopos prováveis

Para MVP read-only:

```text
read_user
read_api
read_repository
```

Para ações futuras:

```text
api
write_repository
```

---

## Épico 4 — Profiles

### Objetivo

Permitir alternar entre GitLab empresarial e pessoal.

### Tasks

- [ ] Criar modelo de profile.
- [ ] Criar seletor de profile no TUI.
- [ ] Suportar `default_profile`.
- [ ] Suportar flag `--profile`.
- [ ] Mostrar profile atual na UI.
- [ ] Listar profiles configurados.
- [ ] Criar comando para adicionar profile.
- [ ] Criar comando para remover profile.
- [ ] Criar comando para renomear profile.
- [ ] Criar comando para testar profile.
- [ ] Persistir últimos projetos por profile.
- [ ] Persistir branches recentes por profile.

### Exemplo de uso

```bash
gzlab --profile empresa
gzlab --profile pessoal
```

---

## Épico 5 — Cliente GitLab

### Objetivo

Criar uma camada limpa para conversar com a API do GitLab.

### Tasks

- [ ] Criar interface `GitLabClient`.
- [ ] Implementar cliente real.
- [ ] Criar método para buscar usuário atual.
- [ ] Criar método para buscar projeto por path.
- [ ] Criar método para listar MRs.
- [ ] Criar método para buscar detalhe de MR.
- [ ] Criar método para listar commits da MR.
- [ ] Criar método para listar arquivos alterados.
- [ ] Criar método para buscar diff.
- [ ] Criar método para buscar pipelines.
- [ ] Criar método para buscar jobs.
- [ ] Criar método para buscar logs de job.
- [ ] Criar método para retry job.
- [ ] Criar método para retry pipeline.
- [ ] Criar método para cancelar pipeline.
- [ ] Criar método para aprovar MR.
- [ ] Criar método para remover aprovação.
- [ ] Criar método para listar discussões.
- [ ] Criar método para responder discussão.
- [ ] Criar método para resolver discussão.

### Requisito técnico

A camada de UI não deve chamar diretamente o SDK do GitLab.

Sempre passar por uma camada de service/client própria.

---

## Épico 6 — Detecção de Projeto Local

### Objetivo

Ao abrir o TUI dentro de um repositório Git, detectar automaticamente o projeto GitLab.

### Tasks

- [ ] Detectar se diretório atual é repo Git.
- [ ] Ler remote `origin`.
- [ ] Parsear URL SSH do GitLab.
- [ ] Parsear URL HTTPS do GitLab.
- [ ] Extrair host.
- [ ] Extrair namespace/project.
- [ ] Identificar profile compatível com host.
- [ ] Buscar projeto na API.
- [ ] Mostrar erro amigável se não encontrar.
- [ ] Mostrar erro se não houver profile para o host.
- [ ] Detectar branch atual.
- [ ] Buscar MR associada à branch atual.
- [ ] Mostrar dashboard contextual do projeto.

### Exemplo

```text
Project: api-protocolo-cadastros
Branch: feature-PD-26527
MR: !250 open
Pipeline: passed
```

---

## Épico 7 — Dashboard Inicial

### Objetivo

Criar uma tela inicial útil e rápida.

### Seções

- profile atual;
- usuário autenticado;
- últimos projetos acessados;
- branches recentes;
- MRs criadas por mim;
- MRs atribuídas a mim;
- MRs para revisar;
- pipelines falhadas recentes.

### Tasks

- [ ] Criar layout base.
- [ ] Criar navegação por seções.
- [ ] Criar card de profile.
- [ ] Criar card de projetos recentes.
- [ ] Criar card de branches recentes.
- [ ] Criar card de MRs.
- [ ] Criar card de pipelines.
- [ ] Criar loading state.
- [ ] Criar empty state.
- [ ] Criar error state.
- [ ] Criar refresh manual.
- [ ] Criar atalhos principais.
- [ ] Criar ajuda com `?`.

---

## Épico 8 — Projetos Recentes

### Objetivo

Permitir voltar rapidamente a projetos usados anteriormente.

### Tasks

- [ ] Registrar projeto quando o usuário abrir.
- [ ] Salvar histórico por profile.
- [ ] Salvar timestamp do último acesso.
- [ ] Mostrar lista ordenada por recência.
- [ ] Permitir remover projeto da lista.
- [ ] Permitir favoritar projeto.
- [ ] Permitir abrir projeto.
- [ ] Permitir copiar URL.
- [ ] Permitir abrir no navegador.
- [ ] Permitir buscar projeto por texto.

### Dados locais

```yaml
recent_projects:
  empresa:
    - path: atendimento/protocolo/api-protocolo-cadastros
      last_accessed_at: "2026-07-01T20:00:00-03:00"
```

---

## Épico 9 — Branches Recentes

### Objetivo

Permitir retomar rapidamente branches usadas ou comentadas recentemente.

### Interpretação do recurso

Branches recentes podem vir de:

- branch atual detectada localmente;
- branches abertas em MRs visualizadas;
- branches relacionadas a comentários/discussões;
- branches acessadas manualmente no TUI;
- branches usadas em checkout pelo app.

### Tasks

- [ ] Criar modelo de branch recente.
- [ ] Salvar branch recente por profile.
- [ ] Salvar branch recente por projeto.
- [ ] Mostrar branch, projeto, MR associada e último acesso.
- [ ] Abrir branch.
- [ ] Abrir MR da branch.
- [ ] Abrir diff da branch/MR.
- [ ] Fazer checkout local da branch.
- [ ] Remover branch do histórico.
- [ ] Buscar branch.
- [ ] Filtrar por projeto.
- [ ] Filtrar por prefixo, por exemplo `feature-PD`.

### Exemplo

```text
Recent Branches
────────────────────────────────────────
feature-PD-26527   api-protocolo-cadastros       !250
fix-sidecar-openapi app-documentos               !88
main              protocolo-model-commons        -
```

---

## Épico 10 — Busca Global

### Objetivo

Permitir pesquisar rapidamente projetos, MRs e branches.

### Tasks

- [ ] Criar entrada de busca com `/`.
- [ ] Buscar projetos.
- [ ] Buscar MRs.
- [ ] Buscar branches.
- [ ] Filtrar resultados por tipo.
- [ ] Permitir navegar por resultados.
- [ ] Permitir abrir resultado.
- [ ] Criar debounce.
- [ ] Criar loading parcial.
- [ ] Criar histórico de buscas.
- [ ] Criar command palette futura.

### Tipos iniciais

```text
project
merge_request
branch
pipeline
```

---

## Épico 11 — Listagem de Merge Requests

### Objetivo

Listar MRs de forma clara e produtiva.

### Filtros

- minhas MRs;
- MRs atribuídas a mim;
- MRs para revisar;
- MRs abertas do projeto;
- MRs com pipeline falhando;
- MRs draft;
- MRs ready;
- MRs por branch;
- MRs por autor;
- MRs por label.

### Tasks

- [ ] Criar tela de lista de MRs.
- [ ] Mostrar ID da MR.
- [ ] Mostrar título.
- [ ] Mostrar branch source.
- [ ] Mostrar branch target.
- [ ] Mostrar status.
- [ ] Mostrar pipeline.
- [ ] Mostrar approvals.
- [ ] Mostrar draft/ready.
- [ ] Mostrar autor.
- [ ] Criar filtros rápidos.
- [ ] Criar busca na lista.
- [ ] Criar refresh.
- [ ] Abrir detalhe da MR.

### Exemplo

```text
!250  PD-26527 Ajusta cadastro   ready   pipeline: passed   approvals: 2/2
!91   Sidecar forwarding fix     draft   pipeline: failed   approvals: 0/1
```

---

## Épico 12 — Detalhe de Merge Request

### Objetivo

Mostrar uma visão completa da MR.

### Seções

- overview;
- descrição;
- branches;
- autor;
- reviewers;
- assignees;
- labels;
- commits;
- pipeline;
- approvals;
- discussões;
- arquivos alterados;
- ações.

### Tasks

- [ ] Criar tela de detalhe.
- [ ] Carregar dados principais.
- [ ] Carregar pipeline principal.
- [ ] Carregar approvals.
- [ ] Carregar discussões.
- [ ] Carregar commits.
- [ ] Carregar arquivos alterados.
- [ ] Mostrar estado draft/ready.
- [ ] Mostrar se há conflito.
- [ ] Mostrar se pode fazer merge.
- [ ] Mostrar motivo de bloqueio da MR.
- [ ] Abrir no navegador.
- [ ] Copiar link.
- [ ] Copiar resumo.

### Recurso importante

Criar uma área chamada:

```text
Why blocked?
```

Exemplo:

```text
MR blocked because:
- pipeline failed
- 1 unresolved thread
- missing approval
```

---

## Épico 13 — Diff Viewer

### Objetivo

Criar uma experiência forte de review no terminal.

### Modos de visualização

#### Modo lado a lado

```text
Original                         Alterado
──────────────────────────────   ──────────────────────────────
return oldValue;                 if value == nil {
                                  return defaultValue
                                }
                                return value
```

#### Modo unificado

```diff
- return oldValue;
+ if value == nil {
+   return defaultValue
+ }
+ return value
```

### Configuração

```yaml
diff:
  mode: side_by_side
  ignore_whitespace: false
  context_lines: 3
```

### Tasks

- [ ] Buscar lista de arquivos alterados.
- [ ] Renderizar arquivos em painel lateral.
- [ ] Buscar diff de arquivo.
- [ ] Parsear diff.
- [ ] Renderizar modo unificado.
- [ ] Renderizar modo lado a lado.
- [ ] Alternar modo com atalho.
- [ ] Persistir preferência.
- [ ] Navegar entre arquivos.
- [ ] Navegar entre hunks.
- [ ] Buscar texto no diff.
- [ ] Expandir contexto.
- [ ] Esconder whitespace.
- [ ] Copiar trecho.
- [ ] Abrir arquivo no editor.
- [ ] Abrir arquivo no navegador.
- [ ] Mostrar loading por arquivo.
- [ ] Mostrar erro se diff for grande demais.
- [ ] Criar fallback para diff grande.

### Atalhos sugeridos

```text
j/k       navegar linhas
n         próximo hunk
p         hunk anterior
f         buscar arquivo
/         buscar no diff
w         toggle whitespace
s         toggle side-by-side/unified
e         abrir no editor
o         abrir no navegador
y         copiar trecho/link
Esc       voltar
```

---

## Épico 14 — Pipeline da MR

### Objetivo

Visualizar pipeline de forma clara.

### Tasks

- [ ] Buscar pipeline principal da MR.
- [ ] Mostrar status geral.
- [ ] Mostrar duração.
- [ ] Mostrar ref/branch.
- [ ] Mostrar usuário que iniciou.
- [ ] Mostrar data.
- [ ] Listar stages.
- [ ] Listar jobs por stage.
- [ ] Abrir job.
- [ ] Retry pipeline.
- [ ] Cancelar pipeline.
- [ ] Atualizar status em tempo real ou por refresh manual.
- [ ] Mostrar jobs manuais.

### Exemplo

```text
Pipeline #18392 — failed

build       passed    1m22s
test        failed    4m10s
quality     skipped
deploy-dev  manual
```

---

## Épico 15 — Logs de Job

### Objetivo

Permitir entender rapidamente por que um job falhou.

### Tasks

- [ ] Buscar log completo do job.
- [ ] Renderizar log com scroll.
- [ ] Criar busca textual.
- [ ] Criar highlight de termos importantes.
- [ ] Pular para próximo erro provável.
- [ ] Copiar linha.
- [ ] Copiar bloco.
- [ ] Salvar log em arquivo.
- [ ] Abrir log no editor.
- [ ] Retry job.
- [ ] Mostrar status do job.
- [ ] Modo follow para job rodando.
- [ ] Tratar logs muito grandes.
- [ ] Baixar log paginado, se necessário.

### Termos para destaque

```text
ERROR
FAILED
Exception
BUILD FAILURE
Compilation failure
Cannot find symbol
npm ERR!
AssertionError
Tests run:
Caused by:
```

### Atalhos sugeridos

```text
/       buscar
n       próximo match
N       match anterior
e       próximo erro
r       retry job
s       salvar log
y       copiar linha
f       follow
Esc     voltar
```

---

## Épico 16 — Discussões e Comentários

### Objetivo

Mostrar comentários e threads da MR.

### Tasks

- [ ] Listar discussões da MR.
- [ ] Separar resolvidas e não resolvidas.
- [ ] Mostrar arquivo e linha quando existir.
- [ ] Abrir diff no ponto do comentário.
- [ ] Responder thread.
- [ ] Resolver thread.
- [ ] Reabrir thread.
- [ ] Criar comentário geral.
- [ ] Criar comentário inline no futuro.
- [ ] Abrir editor externo para comentário longo.
- [ ] Mostrar autor e data.

### Observação

Para MVP inicial, pode começar read-only.

Ações de escrita entram depois.

---

## Épico 17 — Ações de MR

### Objetivo

Permitir operar MRs pelo TUI.

### Tasks

- [ ] Aprovar MR.
- [ ] Remover aprovação.
- [ ] Marcar como draft.
- [ ] Marcar como ready.
- [ ] Fazer merge.
- [ ] Confirmar antes de merge.
- [ ] Copiar link.
- [ ] Abrir no navegador.
- [ ] Checkout branch.
- [ ] Retry pipeline.
- [ ] Cancelar pipeline.
- [ ] Rodar job manual.
- [ ] Adicionar reviewer no futuro.
- [ ] Adicionar assignee no futuro.
- [ ] Adicionar label no futuro.

### Segurança UX

Ações destrutivas devem pedir confirmação:

- merge;
- cancelar pipeline;
- deletar branch;
- remover approval;
- fechar MR;
- deletar algo.

---

## Épico 18 — Checkout de Branch/MR

### Objetivo

Permitir sair do TUI com a branch correta localmente.

### Tasks

- [ ] Detectar repo local.
- [ ] Verificar mudanças locais.
- [ ] Fazer fetch.
- [ ] Verificar se branch local já existe.
- [ ] Criar branch local rastreando remote.
- [ ] Fazer checkout.
- [ ] Mostrar erro se houver conflito.
- [ ] Permitir abortar.
- [ ] Atualizar branch recente após checkout.

### Fluxo

```text
MR !250
Source branch: feature-PD-26527

Action: Checkout branch

Checks:
✅ repo local detectado
✅ sem alterações locais conflitantes
✅ branch remota encontrada

Resultado:
Switched to feature-PD-26527
```

---

## Épico 19 — Workspace Multi-Repo

### Objetivo

Agrupar múltiplas MRs de uma tarefa.

### Tasks

- [ ] Criar modelo de workspace.
- [ ] Criar workspace manualmente.
- [ ] Adicionar MR ao workspace.
- [ ] Remover MR do workspace.
- [ ] Buscar MRs por prefixo de branch.
- [ ] Sugerir workspace baseado em branch atual.
- [ ] Mostrar status consolidado.
- [ ] Mostrar pipelines consolidadas.
- [ ] Mostrar approvals consolidados.
- [ ] Mostrar threads pendentes.
- [ ] Gerar resumo copiável.
- [ ] Salvar workspace localmente.
- [ ] Abrir workspace pelo dashboard.

### Exemplo de workspace

```yaml
workspaces:
  PD-26527:
    profile: empresa
    merge_requests:
      - project: atendimento/protocolo/protocolo-model-commons
        iid: 1475
      - project: atendimento/protocolo/cadastros/api-protocolo-cadastros-dados
        iid: 91
      - project: atendimento/protocolo/cadastros/api-protocolo-cadastros
        iid: 250
```

---

## Épico 20 — Resumo Copiável

### Objetivo

Gerar texto pronto para enviar no Google Chat, Slack ou Markdown.

### Tasks

- [ ] Criar resumo de uma MR.
- [ ] Criar resumo de workspace.
- [ ] Criar resumo de pipelines.
- [ ] Criar resumo de status final.
- [ ] Permitir escolher formato.
- [ ] Copiar para clipboard.
- [ ] Salvar em arquivo.
- [ ] Criar template configurável.

### Formatos

#### Google Chat / Slack

```text
Estado do PD-26527

✅ protocolo-model-commons !1475 — pipeline passou
✅ api-protocolo-cadastros-dados !91 — aprovado
🟡 api-protocolo-cadastros !250 — pipeline rodando
```

#### Markdown

```markdown
## Estado do PD-26527

| Repo | MR | Status |
|---|---:|---|
| protocolo-model-commons | !1475 | ✅ pipeline passou |
| api-protocolo-cadastros-dados | !91 | ✅ aprovado |
| api-protocolo-cadastros | !250 | 🟡 pipeline rodando |
```

---

## Épico 21 — Cache Local

### Objetivo

Melhorar performance e reduzir chamadas à API.

### Tasks

- [ ] Definir diretório de cache.
- [ ] Cachear projetos recentes.
- [ ] Cachear dados básicos de usuários.
- [ ] Cachear reviewers recentes.
- [ ] Cachear branches recentes.
- [ ] Cachear lista de MRs por curto período.
- [ ] Invalidar cache manualmente.
- [ ] Criar TTL por tipo de dado.
- [ ] Evitar cache de token.
- [ ] Evitar cache de logs sensíveis por padrão.

### Diretório sugerido

```text
~/.cache/gitlab-tui/
```

---

## Épico 22 — UI Base

### Objetivo

Criar a estrutura visual e de navegação.

### Tasks

- [ ] Criar layout de três áreas.
- [ ] Criar header.
- [ ] Criar footer com atalhos.
- [ ] Criar painel lateral.
- [ ] Criar painel principal.
- [ ] Criar modal de confirmação.
- [ ] Criar modal de erro.
- [ ] Criar input de busca.
- [ ] Criar command palette no futuro.
- [ ] Criar loading spinner.
- [ ] Criar empty state.
- [ ] Criar sistema de tema.
- [ ] Criar suporte a resize do terminal.
- [ ] Criar suporte opcional a mouse.

### Layout base

```text
┌ GitLab TUI ─────────────────────────────────────────┐
│ Profile: empresa | Project: api-protocolo-cadastros │
├───────────────┬─────────────────────────────────────┤
│ Navigation    │ Main Content                         │
│               │                                      │
│ MRs           │                                      │
│ Pipelines     │                                      │
│ Branches      │                                      │
│ Projects      │                                      │
├───────────────┴─────────────────────────────────────┤
│ ? help | / search | r refresh | q quit               │
└──────────────────────────────────────────────────────┘
```

---

## Épico 23 — Atalhos

### Objetivo

Criar uma navegação previsível.

### Atalhos globais

```text
?       ajuda
/       busca
r       refresh
q       sair
Esc     voltar
Enter   abrir
o       abrir no browser
y       copiar link/resumo
Tab     próximo painel
Shift+Tab painel anterior
```

### Atalhos de MR

```text
d       diff
p       pipeline
c       comentários
a       approve
m       merge
b       checkout branch
```

### Atalhos de pipeline

```text
r       retry job
R       retry pipeline
x       cancelar pipeline
l       abrir log
```

### Atalhos de diff

```text
n       próximo hunk
p       hunk anterior
s       alternar side-by-side/unified
w       toggle whitespace
e       abrir editor
```

---

## Épico 24 — CLI Complementar

### Objetivo

Permitir algumas ações sem abrir o TUI.

### Comandos iniciais

```bash
gzlab
gzlab auth login
gzlab auth status
gzlab profile list
gzlab mr list
gzlab mr view 250
gzlab mr checkout 250
gzlab pipeline list
gzlab pipeline logs <job-id>
```

### Tasks

- [ ] Criar parser CLI.
- [ ] Criar comando raiz.
- [ ] Criar subcomandos de auth.
- [ ] Criar subcomandos de profile.
- [ ] Criar subcomandos de MR.
- [ ] Criar subcomandos de pipeline.
- [ ] Reutilizar services do TUI.
- [ ] Criar output simples.
- [ ] Criar output JSON no futuro.

---

## Épico 25 — Testes

### Objetivo

Garantir estabilidade do app.

### Tasks

- [ ] Testar parser de config.
- [ ] Testar parser de Git remote.
- [ ] Testar seleção de profile.
- [ ] Testar client GitLab com mock.
- [ ] Testar services de MR.
- [ ] Testar services de pipeline.
- [ ] Testar parser de diff.
- [ ] Testar parser de log.
- [ ] Testar cache.
- [ ] Testar atalhos principais.
- [ ] Criar fixtures de resposta GitLab.
- [ ] Criar snapshots de renderização se a stack permitir.

---

## Épico 26 — Distribuição

### Objetivo

Permitir instalar facilmente.

### Tasks

- [ ] Gerar binário para macOS.
- [ ] Gerar binário para Linux.
- [ ] Gerar binário para Windows/WSL.
- [ ] Criar release no GitLab/GitHub.
- [ ] Criar install script.
- [ ] Criar Homebrew tap no futuro.
- [ ] Criar documentação de instalação.
- [ ] Criar changelog.
- [ ] Criar versionamento semântico.

---

# 7. Ordem Recomendada de Desenvolvimento

## Fase 1 — Base utilizável

1. Setup do projeto.
2. Configuração local.
3. Auth por token.
4. Profiles.
5. Cliente GitLab.
6. Detecção de projeto local.
7. Dashboard simples.
8. Listagem de MRs.
9. Detalhe básico de MR.

Resultado esperado:

```text
Consigo abrir o TUI dentro de um repo e ver a MR da branch atual.
```

---

## Fase 2 — Valor real para review

1. Listar arquivos alterados.
2. Renderizar diff unificado.
3. Renderizar diff lado a lado.
4. Navegação por hunks.
5. Busca no diff.
6. Abrir arquivo no editor.
7. Configurar modo de diff.

Resultado esperado:

```text
Consigo revisar código pelo terminal.
```

---

## Fase 3 — CI/CD

1. Buscar pipeline da MR.
2. Listar jobs.
3. Abrir logs.
4. Buscar no log.
5. Pular para erro.
6. Retry job.
7. Retry pipeline.

Resultado esperado:

```text
Consigo entender e reexecutar pipeline quebrada pelo terminal.
```

---

## Fase 4 — Histórico e produtividade

1. Projetos recentes.
2. Branches recentes.
3. Busca global.
4. Resumo copiável.
5. Workspace multi-repo.

Resultado esperado:

```text
Consigo gerenciar uma tarefa com vários repositórios sem abrir várias abas.
```

---

## Fase 5 — Review actions

1. Aprovar MR.
2. Comentar MR.
3. Resolver thread.
4. Checkout branch.
5. Marcar draft/ready.
6. Merge com confirmação.

Resultado esperado:

```text
Consigo fazer review completo pelo terminal.
```

---

# 8. Modelo de Dados Local

## Profile

```go
type Profile struct {
    Name     string
    Host     string
    TokenEnv string
}
```

## RecentProject

```go
type RecentProject struct {
    Profile        string
    ProjectID      int
    Path           string
    Name           string
    WebURL         string
    LastAccessedAt time.Time
}
```

## RecentBranch

```go
type RecentBranch struct {
    Profile        string
    ProjectID      int
    ProjectPath    string
    BranchName     string
    MergeRequestIID *int
    LastAccessedAt time.Time
}
```

## Workspace

```go
type Workspace struct {
    Name          string
    Profile       string
    MergeRequests []WorkspaceMR
}
```

## WorkspaceMR

```go
type WorkspaceMR struct {
    ProjectPath string
    IID         int
}
```

---

# 9. Principais Telas

## 9.1 Dashboard

```text
Dashboard
├── Profile atual
├── Recent Projects
├── Recent Branches
├── My Merge Requests
├── Review Requests
└── Failed Pipelines
```

## 9.2 Recent Branches

```text
Recent Branches
├── Branch
├── Projeto
├── MR associada
├── Último acesso
└── Ações
```

## 9.3 MR Detail

```text
Merge Request
├── Overview
├── Diff
├── Pipeline
├── Discussions
├── Commits
└── Actions
```

## 9.4 Diff

```text
Diff
├── Arquivos alterados
├── Visualização lado a lado
├── Visualização unificada
├── Hunks
└── Busca
```

## 9.5 Pipeline

```text
Pipeline
├── Status geral
├── Stages
├── Jobs
└── Actions
```

## 9.6 Job Log

```text
Job Log
├── Log
├── Busca
├── Erros prováveis
├── Retry
└── Exportar
```

---

# 10. Melhorias Futuras

## Issues

- listar issues;
- abrir issue;
- comentar issue;
- criar issue;
- fechar issue;
- filtrar por labels;
- filtrar por milestone.

## Boards

- Kanban simples;
- mover cards;
- visão por labels.

## Command Palette

```text
Ctrl+K
> approve merge request
> retry failed jobs
> checkout branch
> copy MR summary
```

## IA opcional no futuro

Não entra no MVP.

Possibilidades futuras:

- resumir MR;
- explicar pipeline quebrada;
- sugerir causa provável de falha;
- gerar resumo de changes;
- gerar mensagem de status.

---

# 11. Riscos Técnicos

## Diff grande

Problema:

- diffs muito grandes podem travar ou ficar ruins no terminal.

Mitigações:

- lazy loading;
- limite por arquivo;
- aviso para diff gigante;
- abrir no navegador/editor como fallback.

## Logs grandes

Problema:

- logs de CI podem ser enormes.

Mitigações:

- streaming;
- busca incremental;
- truncamento com aviso;
- opção de salvar em arquivo;
- abrir no editor.

## GitLab self-hosted antigo

Problema:

- APIs podem variar por versão.

Mitigações:

- detectar versão;
- tratar erros;
- criar fallback;
- documentar versão mínima.

## Token e segurança

Problema:

- risco de expor token.

Mitigações:

- token via env;
- mascarar logs;
- keychain no futuro;
- nunca imprimir token.

---

# 12. Definition of Done Inicial

O MVP 1 pode ser considerado pronto quando:

- [ ] O usuário consegue configurar um profile com host e token.
- [ ] O usuário consegue alternar entre profile empresarial e pessoal.
- [ ] O app detecta o projeto atual pelo Git remote.
- [ ] O app detecta a branch atual.
- [ ] O app encontra a MR associada à branch.
- [ ] O app lista MRs do projeto.
- [ ] O app abre o detalhe da MR.
- [ ] O app mostra status da pipeline.
- [ ] O app mostra últimos projetos acessados.
- [ ] O app mostra branches recentes.
- [ ] O app permite abrir MR no navegador.
- [ ] O app permite copiar link da MR.
- [ ] O app tem navegação básica estável.

---

# 13. Primeira Slice Recomendada

A menor entrega valiosa seria:

```text
Abrir o TUI dentro de um repo GitLab e ver a MR da branch atual.
```

Tasks dessa slice:

- [ ] Setup Go + Bubble Tea.
- [ ] Config YAML.
- [ ] Token via env.
- [ ] Profile default.
- [ ] Parse do remote origin.
- [ ] Detectar branch atual.
- [ ] Buscar projeto no GitLab.
- [ ] Buscar MR da branch.
- [ ] Renderizar tela simples.
- [ ] Mostrar pipeline status.
- [ ] Abrir MR no navegador.
- [ ] Copiar link da MR.

Tela esperada:

```text
GitLab TUI

Profile: empresa
Project: api-protocolo-cadastros
Branch: feature-PD-26527

MR: !250 — PD-26527 Ajusta cadastro
Status: opened
Pipeline: passed
Approvals: 2/2
Threads: 0 unresolved

Actions:
[d] diff
[p] pipeline
[o] open browser
[y] copy link
[q] quit
```

---

# 14. Nome Provisório

Ideias:

- `glt`
- `labtui`
- `gitlab-cockpit`
- `mrctl`
- `labdash`
- `tuilab`

Sugestão prática:

```text
glt
```

Curto, fácil de digitar e com cara de ferramenta de terminal.

---

# 15. Resumo Executivo

Este projeto deve começar como um cockpit de Merge Requests e Pipelines para GitLab.

A melhor primeira versão não é um cliente completo do GitLab, mas sim uma ferramenta que resolve o fluxo diário do dev:

1. abrir repo;
2. detectar branch;
3. encontrar MR;
4. ver status;
5. revisar diff;
6. abrir pipeline;
7. entender logs;
8. copiar resumo;
9. alternar entre GitLab empresa e pessoal.

A partir disso, evolui para workspace multi-repo, review actions, criação de MR e, por último, issues/boards.
