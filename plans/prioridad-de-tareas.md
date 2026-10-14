# Feature: Prioridad de tareas

## Encargo y criterios de aceptación

Encargo literal: «Cada tarea tiene una prioridad: alta, media o baja. Se elige al crearla y se puede cambiar después. Las tareas que ya existen pasan a media.»

Criterios de aceptación (explícitos):

1. Toda tarea tiene exactamente una prioridad entre `alta`, `media` y `baja`.
2. El formulario de creación permite elegir la prioridad; la tarea se guarda con la elegida.
3. El formulario de edición permite cambiar la prioridad de una tarea existente.
4. Tras aplicar la migración, todas las tareas existentes tienen prioridad `media`.

Criterios inferidos (no pedidos literalmente, necesarios para un comportamiento coherente):

5. La prioridad es visible en la lista de tareas, como texto (no solo color).
6. En el formulario de creación la opción preseleccionada es `media`.
7. Si una petición de creación no envía prioridad, la tarea se crea con `media`.
8. Un valor distinto de `alta`, `media` o `baja` se rechaza con 422 y el mensaje «La prioridad no es válida.», sin modificar datos.
9. En la edición, si no se envía prioridad se conserva la actual; editar no altera el estado de completada (comportamiento ya existente).
10. Si la creación falla por validación, el formulario re-renderizado conserva la prioridad elegida.

## Comportamiento actual y propuesta

Actual: `models.Task` tiene `ID`, `Title`, `Completed` y timestamps. La lista (`GET /`) muestra título y estado; el alta (`POST /tasks`) solo enlaza `title`; la edición (`PUT /tasks/{id}`) enlaza solo `title` mediante un struct de entrada y conserva el resto.

Propuesta: añadir la columna `priority` (texto, NOT NULL, default `'media'`) que rellena las filas existentes al migrar. El modelo valida el valor y, mediante el callback `BeforeValidate` de pop, convierte una prioridad vacía en `media`. Los formularios de alta y edición añaden un `<select name="priority">`. La lista muestra una etiqueta con la prioridad.

## Alcance

Incluido:

- Migración que añade `tasks.priority` con default `media`.
- Campo, constantes, validación y etiqueta legible en el modelo.
- Enlace de `priority` en creación y edición.
- Selector en `tasks/index.plush.html` y `tasks/edit.plush.html`; etiqueta de prioridad en la lista.
- CSS mínimo para el selector y la etiqueta.
- Tests de modelo y de acciones.

Excluido:

- Ordenar o filtrar por prioridad (el orden sigue siendo `created_at asc, id asc`).
- Cambiar la prioridad desde la lista sin pasar por la edición, o con JavaScript.
- Restricción CHECK en base de datos (la validación vive en el modelo, como la del título).
- Cambios en la migración de seed existente o en los fixtures (el default de la columna cubre ambos).

## Diseño por capas

**Persistencia.** Nueva migración fizz `migrations/<timestamp>_add_priority_to_tasks.{up,down}.fizz`:

- up: `add_column("tasks", "priority", "string", {"size": 10, "default": "media"})` (fizz crea columnas NOT NULL por defecto; el default rellena las filas existentes, incluidas las del seed).
- down: `drop_column("tasks", "priority")`.

Usa un timestamp posterior a `20260901000001` (p. ej. `20261014000000`).

**Modelo (`models/task.go`).**

- Constantes `PriorityHigh = "alta"`, `PriorityMedium = "media"`, `PriorityLow = "baja"` y una lista ordenada `Priorities = []string{PriorityHigh, PriorityMedium, PriorityLow}` para iterar en las plantillas.
- Campo `Priority string \`json:"priority" db:"priority" form:"priority"\``.
- `BeforeValidate(tx *pop.Connection) error`: si `strings.TrimSpace(t.Priority) == ""`, asigna `PriorityMedium`. Solo se ejecuta en `ValidateAndCreate/Update/Save`; `TasksToggle` usa `tx.Update` y no lo necesita porque la tarea ya viene de la base de datos con prioridad.
- En `Validate`: si `t.Priority` no está en `Priorities`, `verrs.Add("priority", "La prioridad no es válida.")`.
- Función de paquete `PriorityLabel(p string) string` que devuelve `"Alta"`, `"Media"` o `"Baja"` (cadena vacía si no es válida), y método `func (t Task) PriorityLabel() string { return PriorityLabel(t.Priority) }`. La lista usa el método; las opciones del `<select>` usan la función, registrada en `render.Helpers` como `priorityLabel`.

**Acciones (`actions/tasks.go`).**

- `TasksIndex`: `c.Set("priorities", models.Priorities)` además de lo actual. `&models.Task{}` pasa a `&models.Task{Priority: models.PriorityMedium}` para preseleccionar `media`.
- `TasksCreate`: sin cambios en el enlace (`c.Bind(task)` ya recogerá `priority` por la etiqueta `form`). En la rama de error, añadir `c.Set("priorities", models.Priorities)`.
- `TasksEdit`: `c.Set("priorities", models.Priorities)`.
- `TasksUpdate`: el struct de entrada añade `Priority string \`form:"priority" json:"priority"\``. Tras enlazar, si `strings.TrimSpace(input.Priority) != ""`, asignar `task.Priority = strings.TrimSpace(input.Priority)`; si no, conservar la actual. En la rama de error, `c.Set("priorities", models.Priorities)`. Actualizar el comentario de la función («actualiza el título y la prioridad, conservando los demás datos»).

Para no repetir `c.Set("priorities", …)` en cuatro sitios basta con repetirlo: es una línea y sigue el estilo existente (las acciones ya repiten `c.Set` de `tasks`/`errors`). No introducir middleware.

**Render (`actions/render.go`).** Registrar el helper `"priorityLabel": models.PriorityLabel` en `render.Helpers` (importa `todo/models`).

**Plantillas.**

`templates/tasks/index.plush.html`:

- En cada `<li>`, tras `<span class="task-title">…</span>` (y antes del estado), añadir
  `<span class="task-priority task-priority-<%= t.Priority %>">Prioridad <%= t.PriorityLabel() %></span>` con el texto «Prioridad» dentro de un `<span class="sr-only">` para lectores de pantalla y la etiqueta visible (`Alta`/`Media`/`Baja`). No meter la etiqueta dentro de `.task-title`: `app.js` usa su `textContent` para el `aria-label` del botón de completar.
- En el formulario `.new-task`, dentro de `.new-task-row` entre el input y el botón: `<label for="task-priority" class="sr-only">Prioridad</label>` y `<select id="task-priority" name="priority">` con un `<option>` por cada `p in priorities`, `value="<%= p %>"`, texto `<%= priorityLabel(p) %>`, y `selected` si `p == task.Priority`.
- Si `errors.Get("priority")` tiene mensajes, mostrarlos en el mismo bloque `.field-error` que los de `title` (iterar ambos).

`templates/tasks/edit.plush.html`:

- Tras el bloque del título (antes de `.edit-task-actions`), añadir `<label for="task-priority">Prioridad</label>` y el mismo `<select>` con la opción actual seleccionada.
- Mostrar errores de `priority` en un `<p id="priority-error" class="field-error" role="alert">` cuando existan.
- Cambiar el texto de ayuda `title-help` a «Hasta 200 caracteres.» (la frase «Los demás datos de la tarea no cambian» deja de ser exacta) — mantener el `id`.
- Ajustar la condición de `aria-invalid`/`aria-describedby` del título para que dependa de `errors.Get("title")` y no de `errors.HasAny()`, de modo que un error de prioridad no marque el título como inválido. Hacer lo mismo en `index.plush.html`.

**CSS (`public/assets/app.css`).**

- `.task`: `grid-template-columns: 44px minmax(0, 1fr) auto auto;` (y `36px minmax(0, 1fr) auto auto` en el media query de 760px), para que la etiqueta ocupe su propia columna antes de `.task-actions`.
- `.task-priority`: `align-self: center`, tamaño de letra pequeño (`0.75rem`), padding `0.15rem 0.5rem`, `border-radius: 999px`, borde `1px solid var(--color-border)`, color `var(--color-muted)`. Variante `.task-priority-alta` con color `var(--color-primary)` y fondo `#fff0eb`; `.task-priority-baja` con fondo `var(--color-sidebar)`. El texto siempre está presente, así que el color es refuerzo, no la única señal.
- `select` dentro de `.new-task-row` y `.edit-task`: reutilizar la tipografía/borde de los inputs existentes (copiar las propiedades de borde, radio y altura mínima de 44px de `.new-task-row input`). Incluir `select:focus-visible` en la regla de foco existente.
- En el media query de 380px, el selector no debe provocar scroll horizontal (dejar que haga wrap junto con el botón).

**JavaScript.** Sin cambios. `app.js` inserta `.task-status` antes de `.task-actions` y es `position: absolute` (sr-only), así que la nueva columna no le afecta.

## Archivos relevantes

| Ruta | Responsabilidad |
|---|---|
| `migrations/20261014000000_add_priority_to_tasks.up.fizz` | Nueva: añade la columna con default `media` |
| `migrations/20261014000000_add_priority_to_tasks.down.fizz` | Nueva: elimina la columna |
| `models/task.go` | Campo, constantes, `BeforeValidate`, validación, etiquetas |
| `models/task_test.go` | Tests de default, validación y persistencia |
| `actions/tasks.go` | Enlace de prioridad en alta/edición y `priorities` en contexto |
| `actions/render.go` | Helper `priorityLabel` |
| `actions/tasks_test.go` | Tests de alta, edición, lista y valores inválidos |
| `templates/tasks/index.plush.html` | Selector en alta y etiqueta en la lista |
| `templates/tasks/edit.plush.html` | Selector en edición |
| `public/assets/app.css` | Estilos de etiqueta y selector |
| `migrations/embed.go`, `fixtures/tasks.toml` | Sin cambios (el embed recoge los `.fizz` nuevos; los fixtures usan el default) |

## Implementación paso a paso

1. Crear las dos migraciones fizz.
2. Modelo: constantes, `Priorities`, campo `Priority`, `PriorityLabel(p string)`, método `(t Task) PriorityLabel()`, `BeforeValidate` y validación.
3. `render.go`: registrar `priorityLabel`.
4. Acciones: cambios descritos en `TasksIndex`, `TasksCreate`, `TasksEdit`, `TasksUpdate`.
5. Plantillas `index` y `edit`.
6. CSS.
7. Tests (ver siguiente sección).
8. Validación: `gofmt -l .`, `go vet ./...`, migraciones de test, `go test -p 1 ./...`, `go build ./...`.

## Tests y validación

Tests de modelo (`models/task_test.go`):

- `Test_Task_Priority_DefaultsToMedium`: crear con solo título → `Priority == "media"` tras `ValidateAndCreate` y tras recargar.
- `Test_Task_Priority_ValidValues`: `alta`, `media`, `baja` se crean sin errores.
- `Test_Task_Priority_Invalid`: `"urgente"` → error en `priority` con «La prioridad no es válida.» y no se crea.
- `Test_Task_PriorityLabel`: etiquetas `Alta`, `Media`, `Baja`.

Tests de acciones (`actions/tasks_test.go`):

- `Test_TasksIndex_DefaultPrioritySelected`: `GET /` contiene `<select` con `name="priority"` y la opción `media` seleccionada.
- `Test_TasksIndex_ShowsPriority`: tarea con `alta` → el cuerpo contiene `task-priority-alta` y `Alta`.
- `Test_TasksCreate_WithPriority`: `POST /tasks` con `title` y `priority=alta` → 303 y tarea con `alta`.
- `Test_TasksCreate_WithoutPriority_DefaultsToMedium`: `POST` con solo `title` → tarea con `media` (los tests de creación existentes ya cubren este camino; este lo hace explícito).
- `Test_TasksCreate_InvalidPriority`: `priority=urgente` → 422, «La prioridad no es válida.», 0 tareas, y el título enviado se conserva en el input.
- `Test_TasksEdit_PreselectsPriority`: tarea `baja` → la opción `baja` aparece seleccionada.
- `Test_TasksUpdate_ChangesPriority`: `PUT` con título y `priority=alta` sobre una tarea `media` completada → 303, prioridad `alta`, sigue completada.
- `Test_TasksUpdate_WithoutPriority_KeepsCurrent`: `PUT` solo con título sobre tarea `baja` → sigue `baja`.
- `Test_TasksUpdate_InvalidPriority`: `priority=urgente` → 422 con el mensaje y prioridad sin cambios.

Los tests existentes deben seguir pasando sin modificaciones; si alguno compara HTML que cambie (p. ej. `aria-describedby="title-error"`), debe seguir siendo cierto con la nueva condición basada en `errors.Get("title")`.

Para comprobar el criterio 4 (filas existentes pasan a `media`) basta con el default de la columna; se verifica manualmente tras `soda migrate` en desarrollo: las tres tareas del seed muestran «Media».

Comandos (requieren PostgreSQL, p. ej. `docker compose up -d db`, y crear `todo_test` si no existe):

```bash
gofmt -l .
go vet ./...
GO_ENV=test soda migrate -e test   # o: go run github.com/gobuffalo/pop/v6/soda@v6.4.1 migrate -e test
GO_ENV=test go test -p 1 ./...
go build ./...
```

## Riesgos y recuperación

- **Valores en español en base de datos.** Se guardan `alta/media/baja` para evitar una capa de traducción; si en el futuro se quieren claves en inglés hará falta otra migración de datos.
- **Default solo en columna y `BeforeValidate`.** Inserciones SQL directas sin `priority` reciben `media` por el default; código que use `tx.Create` sin validar y prioridad vacía insertaría `''` (no hay CHECK). Ningún camino actual lo hace.
- **Orden de migraciones.** El timestamp debe ser posterior al seed para que este ya esté insertado (o no, da igual: el default aplica en ambos casos).
- **Diseño en móvil.** La columna extra estrecha el título; verificar a 375px que no hay scroll horizontal.
- **Rollback.** `soda migrate down` elimina la columna; revertir el commit restaura código y plantillas.
