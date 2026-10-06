# Migration Plan Status

Obiettivo: trasformare il fork di Picture Frame nella base software di una
cornice digitale hardware dedicata, Immich-only, con separazione
`Core ≠ Photo Provider`.

Principio: ogni step è piccolo, verificabile, senza cambi di comportamento
salvo dove dichiarato. Aggiornare questo file a ogni step completato.

## Legenda

- `[x]` completato e verificato
- `[~]` completato ma verifica pendente
- `[ ]` da fare

## Step

### S1 — Seam `providers/` (interfaccia provider) `[x]`

- Nuovo `providers/provider.go`: `Asset` + interfaccia `Provider` (`List`, `Fetch`).
- `internal/library/remote.go`: `Asset` / `RemoteAlbum` diventano alias di
  `providers.Asset` / `providers.Provider`. Nessun cambio di comportamento.
- `internal/library/adapter/immich/immich.go`: assertion
  `var _ providers.Provider = (*Client)(nil)`, commento aggiornato.
- Verifica: `go build -buildvcs=false ./...` OK (via Docker `golang:1`, go1.27.1;
  `-buildvcs=false` necessario per il bind-mount);
  `go vet` + `go test ./providers/... ./internal/library/...` OK
  (library, adapter, immich tutti `ok`).

### S2 — Client Immich via API key `[x]`

- Nuovo `internal/library/adapter/immich/apikey.go`: `APIClient`
  (`BaseURL` + `APIKey` + `AlbumID`, header `x-api-key`, `var _ providers.Provider`).
  Enumerazione via timeline buckets API (stessa del client shared-link, verificata
  contro la OpenAPI live di Immich: `AlbumResponseDto` NON contiene più `assets`,
  `/timeline/bucket` e `/api/assets/{id}/thumbnail` accettano `api_key`).
  ETag gate con 304, thumbnail `size=preview`, nessun retry su 401 (chiave statica).
  Chiave mai in URL o log.
- Config: `ImmichLibraryConfig` + `url` / `api_key` / `album_id` + `UsingAPI()`;
  validazione XOR con shared-link (messaggi: `share_url required` se nessuno,
  `mutually exclusive` se entrambi, `all required` se trio incompleto).
- Wiring `startLibrarySyncer`: usa `APIClient` in api-mode, legacy altrimenti.
- Test `apikey_test.go` (mock HTTP: header richiesto, 401 su chiave errata,
  filtro video, cache 304, fetch preview, errori propagati, trim trailing slash).
- `config.example.toml`: documentate entrambe le modalità.
- NON fatto (volutamente): campi api-key nell'admin UI (DTO + Svelte + regen
  OpenAPI) — la modalità si configura via TOML; l'admin PUT preserva i campi.
- Verifica: `go build ./...` OK, `go vet` pulito, `gofmt` pulito,
  `go test ./providers/... ./internal/library/... ./internal/config/...` tutti `ok`
  (via Docker `golang:1`, `-buildvcs=false`).

### S3 — Cache con metadata + GC `[x]`

- Nuovo `internal/library/manifest.go`: `Manifest` (source of truth di cosa è
  in cache) + `ManifestEntry{id, version, bytes, added_at}`. Sidecar
  `.manifest.json` (dot-convention come aspect/order), scrittura atomica,
  load tollerante (mancante/corroto → vuoto, il syncer ri-adotta da disco).
  `Missing()` (cosa scaricare), `DropMissing()` (entry orfane),
  `TotalBytes()`. `added_at` registrato ora per una futura LRU.
- `Syncer`: campo manifest (default in-memory, nessun file → test esistenti
  intoccati) + `maxBytes`; opzioni `WithManifest` / `WithMaxBytes`.
  Flusso: scan → DropMissing → delete stale → adopt pre-manifest → download
  dei Missing con stop a budget esaurito (skip ≠ failure: status OK, warn in log).
  `download`/`writeAtomic` restituiscono i byte scritti.
- Config: nuova sezione `[cache]` con `max_size` (`ByteSize`: numero o
  B/KB/MB/GB, 0 = illimitato); esempio in `config.example.toml`.
- Wiring `syncOpts`: manifest persistente + budget nel syncer (chiavi API mai loggate).
- Test: `manifest_test.go` (roundtrip, corrupt, version-change, DropMissing,
  adopt senza re-download, stale-drop con re-download, budget skip stabile su
  due cicli, unlimited) + `TestByteSizeParsing` + `TestCacheMaxSizeLoadsFromTOML`.
- Semantica budget documentata: over-budget = subset deterministico (primi N in
  ordine remoto), niente churn delete/re-download. Vera LRU con eviction dei
  file *wanted* rimandata: richiederebbe on-demand fetch (S5); il manifest è
  già sagomato per essa.
- NON fatto (volutamente): `[cache]` nell'admin UI — via TOML; l'admin PUT
  preserva il campo (verificato: `httpapi` e `cmd` test `ok`).
- Verifica: `go build ./...` OK, `go vet` pulito, `gofmt` pulito,
  `go test -count=1` `ok` su providers/library(+adapter,+immich)/config/httpapi/cmd
  (via Docker `golang:1`, `-buildvcs=false`).

### S4 — Scheduler con backoff `[x]`

- `Syncer.Run`: ticker fisso → timer con delay calcolato per ciclo.
  `syncOnce` restituisce `bool` (fallito = list/scan error o download falliti;
  gli skip a budget NON sono fallimenti). Fallimento → backoff esponenziale
  dalla base (default 30s, `WithRetryBase`), raddoppio per failure consecutive,
  jitter in [d/2, d] (`math/rand/v2`), cap all'intervallo: un failure persistente
  converge alla cadenza normale, mai più veloce. Successo → intervallo esatto
  (nessun drift/accumulo come col ticker). Trigger manuale invariato (sync subito).
- First-image-fast: `advance.Next()` al PRIMO download riuscito (non a fine
  album): a cache vuota lo slideshow parte subito, l'ordine di download resta
  quello remoto (= ordine di prima visualizzazione).
- Priorità next-slide: SCELTA DOCUMENTATA di non implementare euristiche ora.
  Il syncer non può conoscere la "next" (vive in planner/filename, S5);
  l'ordine remoto è l'ordine di display per cache fresche e in steady-state i
  nuovi asset si accodano a fine ciclo in ogni caso. La priorità vera arriva con
  `Ensure([current,next])` di S5, che fornisce il segnale. Nessun hook
  speculativo aggiunto (no over-engineering).
- Test: `backoff_internal_test.go` (range per livello, cap, default) +
  `TestSyncRetriesFailuresWithBackoff` / `TestSyncSuccessWaitsFullInterval`
  (synctest, tempo virtuale) + `TestSyncAdvancesOnFirstSuccessDespiteLaterFailure`;
  `fakeRemote` + contatore `lists` e `failIDs`.
- Verifica: `go build ./...` OK, `go vet` pulito, `gofmt` pulito,
  `go test -count=1` `ok` su library(+adapter,+immich)/config/cmd
  (via Docker `golang:1`, `-buildvcs=false`). Nota: `rand.Int63n` non esiste in
  `math/rand/v2`, usato `rand.N`.

### S5 — Preload current+next `[x]`

- `slideplan.Planner.PeekNext()`: sbirciata non-avanzante ( riusa `ensure()`,
  mai nuovo ciclo: a fine plan torna alla prima slide corrente; nil se vuoto).
- `state.ImagePayload.Next` (`json:"next,omitempty"`): hint best-effort.
  `Slideshow.preloadHint` lo compila: nil se uguale alla corrente (plan da una
  slide) o se un'immagine manca in libreria (entry stale che `servable`
  salterebbe).
- Rigenerato il client TS come da pipeline (`go generate` + `npx openapi-ts`:
  `web/openapi.json` + `types.gen.ts` con `next?`). Nota: `npm i` richiede
  `--force` con node v24.14.1 (il repo chiede ^24.15.0).
- Kiosk `Images.svelte`: effect che precarica `next` (`new Image()` + `decode()`
  in cache browser) mentre la corrente è a schermo; il crossfade trova l'immagine
  già pronta. Best-effort, nessun cambio al Fader.
- Garanzia "offline prima della transizione": già coperta da `servable()`
  (solo file locali) + hint filtrato su `lib.Has`; nessun publish di slide
  non disponibili.
- Test: 3 peek (no-advance, wrap-senza-nuovo-ciclo, vuoto) + 3 publish
  (hint, hint assente su singola, hint assente se target cancellato).
- Verifica: `go build` OK, `go test -count=1` `ok` su TUTTI i package
  `internal/...` non-adapter (come `make test`), `npm run check` 0 errori,
  `npm run test:unit` 424/424.

### S6 — Config minima MVP `[x]`

- Nuova sezione canonica `[immich]` (`url`, `api_key`, `album_id`,
  `sync_interval`): i campi api_* di S2 (mai rilasciati) si sono spostati da
  `[library.immich]`, che resta solo shared-link legacy
  (`ImmichLibraryConfig` → `ImmichLibraryShareConfig`). `[library] backend`
  resta il selettore. Validazione XOR riscritta su `Config.validateImmich`
  (stessi messaggi: `or [immich]` / `mutually exclusive` / `all required`).
- `[display]` += `width`, `height` (0 = non specificato; il kiosk web segue il
  viewport, i futuri renderer/thumbnail sizing li useranno). Validazione >= 0.
  `transition` NON aggiunto (volutamente): un knob con un solo valore
  ("fade") non ha valore; arriva col renderer di S7 se serve.
- Kill switch con default invariato (via `defaults()`, nessun breaking):
  `[weather] enabled`, `[wifi] enabled`, `[updater] enabled` (default true =
  comportamento attuale). `false` salta goroutine e connessioni:
  `WeatherEnabled`, `buildWiFiManager` (prima dei mock/validazioni),
  `startUpdater` (ritorna nil). Sensori/MQTT restano opt-in per costruzione
  (lista vuota / bridge spento).
- `config.minimal.example.toml` (nuovo): MVP Immich-only con tutto il resto
  spento; coperto da `TestMinimalExampleLoads`.
- `config.example.toml`: documentati `[immich]`, share legacy, width/height,
  flag enabled.
- NON fatto (volutamente): nuovi campi nell'admin UI (precedente S2/S3:
  TOML-only, l'admin PUT preserva); docs site (`docs/src/.../configuration.md`
  resta corretto: descrive share_url che funziona ancora).
- Verifica: `go build ./...` OK, `go vet ./...` pulito, `gofmt` pulito,
  `go test -count=1` `ok` su TUTTI i package non-adapter + cmd
  (via Docker `golang:1`, `-buildvcs=false`). Aggiornati: `TestValidateLibrary`,
  `TestWeatherEnabled`/`TestBuildWeatherFetcher` (fixture + casi disabled),
  `configdto_test.go` (rename tipo).

### S7 — Interfaccia renderer `[x]` (chirurgica: solo contratti, nessuno spostamento)

- Nuovo `internal/renderer/renderer.go`: `Slide{Names, Next}` + interfaccia
  `Renderer{ Show(Slide) }`. Nessun chiamante ancora (contratto dichiarato per
  il futuro framebuffer/DRM): il doc mappa esplicitamente come il flusso attuale
  lo soddisfa (publish → bus → SSE → kiosk Fader, `/img/`). Niente astrazione
  prematura oltre questo.
- Nuovo `internal/hardware/power/power.go`: interfaccia `Controller`
  (On/Off/State/SetBrightness 0-100, valori fuori range = errore) + `Noop`
  thread-safe (precedente `display.Mock`). Niente sentinel `ErrUnsupported`
  speculativa; niente night-mode/batteria (interfacce future, non metodi).
  Disambiguato da `internal/power` (host reboot/shutdown via logind).
- Sensori: NESSUN nuovo package — `sensors.Source`/`Reading`/`Registry` + mock
  sono già il seam (verificato); duplicarlo sarebbe stato rumore.
- Display power: NESSUNO spostamento — `display.Controller` + `Policy`
  restano dov'è; è già il backend di produzione dell'interfaccia power.
- Verifica: `go build ./...` OK, `go vet` pulito, `gofmt` pulito,
  `go test` `ok` su `hardware/power`; nessun test esistente toccato
  (via Docker `golang:1`, `-buildvcs=false`).

### S8 — Rimozione funzionalità non-MVP `[ ]` (APERTO, utente ha chiesto prima S9)

- Vedi tabella §10 dell'analisi: video, multi-album, Ken-Burns, upload/crop,
  overlay meteo/sensori, split-screen pairing, updater OTA, ecc.

### S9 — Multi-album `[x]`

- Config `[immich]`: `album_id` → `album_ids = [...]` (sostituzione, mai
  rilasciato). Validazione: url + api_key + almeno un ID; XOR con share invariato.
- `APIClient`: un `albumState{id, assets, etag, loaded}` per album in ordine
  config; `List` interroga ogni gate ETag, ri-enumererà solo i cambiati, fonde
  con `mergeAssets` (ordine album, poi timeline; dedupe per ID, first-wins —
  una foto in due album si scarica una volta). Un album in errore fallisce
  l'intera List (il syncer tiene la cache e riprova in backoff). Ordinamento
  globale timeline NON implementato (volutamente): nessun segnale di interleave
  dall'API, l'ordine config è deterministico e documentato.
- `cloneConfig` deep-copia `AlbumIDs` (il test reflect l'ha imposto) + allowlist.
- Wiring: logga gli UUID album (non segreti), mai la chiave.
- Test: fake multi-album (`albums`/`etags` per-album, 404 su sconosciuti) +
  merge+dedupe in ordine, ETag parziale (solo B ri-enumerato), album
  sconosciuto → errore; aggiornati config test + TOML load.
- `config.example.toml` + `config.minimal.example.toml`: `album_ids`.
- Verifica: `go build ./...` OK, `go vet ./...` pulito, `gofmt` pulito,
  `go test -count=1` `ok` su TUTTI i package non-adapter + cmd
  (via Docker `golang:1`, `-buildvcs=false`).

### S10 — Album picker nell'admin UI `[x]`

- **Ribalta la scelta S2** ("NON fatto (volutamente): campi api-key nell'admin
  UI — la modalità si configura via TOML"). Motivo: senza UI, `album_ids`
  obbligava a copiare UUID a mano dal browser Immich. Ora il frame li
  elenca e li scegli.
- `immich.ListAlbums(ctx, baseURL, apiKey, httpc)` (nuovo
  `internal/library/adapter/immich/albums.go`): `GET /api/albums`. Funzione di
  package, non metodo di `APIClient` — la discovery deve funzionare *prima*
  che un album sia configurato, e `NewAPIClient` rifiuta `album_ids` vuoto.
  Header `x-api-key` come il resto del client.
- `POST /api/immich/albums` (`internal/httpapi/handlers.go`). POST con body
  opzionale, non GET: salvare la modalità api-key richiede almeno un album
  (`validateImmich`), quindi un endpoint solo-GET sarebbe un deadlock — la
  lista non sarebbe mai raggiungibile da una config vuota. I campi vuoti
  ricadono sulla config salvata. La chiave sta nel body, non nella query
  string.
- `albumListError`: `status 401` nudo (`immich.go`) diventa 401/403/502 con un
  messaggio che dice se il problema è la chiave.
- `ConfigDTO`: `LibraryDTO.ApiKey` (`ImmichAPIKeyDTO`: url, api_key
  write-only + `api_key_set`, album_ids, sync_interval). `applyLibraryDTO`
  cambia firma per coprire `config.Immich`, che sta fuori `LibraryConfig`.
  `toDTO` non restituisce mai la chiave.
- `LibraryCard.svelte`: selettore modalità (API key / Share link) che pulisce
  i campi dell'altra — `validateImmich` li rifiuta insieme. Il picker sposta
  l'ordine di `album_ids` in quello mostrato, che è l'ordine di merge lato
  backend.
- `validate.ts`: `validateImmich` rispecchia la XOR Go. Prima l'UI impediva di
  salvare `backend = "immich"` senza share_url.
- Docs: `manual/photos.md` riscritto (api-key + share), `reference/configuration.md`
  con `[immich]`, `index.mdx` non dice più "never an API key".
- Non è Tier-1: cambiare gli album richiede restart (test dedicato).
- Verifica: `make test` (coverage 93%, Go + 453 Vitest), `make test-e2e`
  (110), `npm run lint`, `npm run check`, `go vet`, `gofmt` puliti.
