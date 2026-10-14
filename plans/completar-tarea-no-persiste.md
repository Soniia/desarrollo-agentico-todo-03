# Bug: Completar una tarea no se guarda y se pierde al recargar

## Encargo y resultado esperado

Encargo: «Cuando marco una tarea como hecha y recargo la página, la tarea vuelve a aparecer sin hacer.»

Resultado esperado:

1. Al pulsar el botón de completar (o reabrir) de una tarea en la lista, el cambio se guarda en la base de datos.
2. Al recargar la página, la tarea conserva el estado que se le dio.
3. La interfaz sigue respondiendo al instante (sin recarga completa) como ahora: tachado, icono, `aria-pressed`, `aria-label` y estado «Completada» para lectores de pantalla.
4. Si el servidor no confirma el cambio, la página no debe quedarse mostrando un estado que no se guardó.

## Evidencia y causa raíz

Hechos observados en el repositorio:

- `templates/tasks/index.plush.html`: el botón de completar está dentro de `<form action="/tasks/<id>/toggle" method="POST" data-task-toggle>` con los campos ocultos `authenticity_token` y `_method=PUT`.
- `actions/app.go`: la única ruta de alternancia es `app.PUT("/tasks/{task_id}/toggle", TasksToggle)`. No existe `GET /tasks/{task_id}/toggle`.
- `public/assets/app.js`: el manejador de `submit` hace `event.preventDefault()` en los formularios `[data-task-toggle]`, actualiza el DOM y después llama a `fetch(form.action);` sin opciones.
- `fetch(url)` sin opciones envía una petición **GET** sin cuerpo. Esa petición no lleva `_method=PUT` ni `authenticity_token`, así que el `MethodOverride` de Buffalo (`buffalo@v1.1.4/method_override.go`, que solo actúa sobre POST con `_method` en el formulario) no la convierte en PUT. El router no tiene ningún GET para esa ruta, así que acaba en `ServeFiles` y responde 404. `TasksToggle` nunca se ejecuta.
- El resultado de `fetch` no se comprueba, de modo que el fallo es silencioso: el DOM muestra la tarea completada, pero la base de datos no cambia, y al recargar vuelve a pendiente.
- `actions/tasks_test.go` (`Test_TasksToggle_CompletesAndReopens`) llama directamente a `PUT /tasks/:id/toggle` y pasa: el servidor funciona; el fallo está solo en el cliente.
- CSRF (`gobuffalo/middleware/csrf`): en métodos no seguros exige el token en el campo de formulario `authenticity_token` o en la cabecera `X-CSRF-Token`. En `GO_ENV=test` el middleware no se aplica, por lo que los tests de acciones no detectan peticiones sin token.

Causa raíz: `public/assets/app.js` envía la alternancia con `fetch(form.action)` (GET sin cuerpo) en lugar de reproducir el envío del formulario (POST con `_method=PUT` y `authenticity_token`).

Incertidumbres:

- Lo anterior está deducido del código, no de una ejecución observada. La reproducción manual de abajo debe confirmarlo antes del arreglo (en la pestaña de red se verá `GET /tasks/<id>/toggle` → 404).
- No hay infraestructura de tests de JavaScript (no existe `package.json` ni runner de navegador), así que el comportamiento completo solo se puede verificar en un navegador.

Reproducción mínima:

1. Arrancar la aplicación en desarrollo con al menos una tarea pendiente (las del seed sirven).
2. Abrir `/`, pulsar el círculo de completar de una tarea pendiente: aparece tachada.
3. Recargar la página: la tarea vuelve a estar pendiente.
4. En las herramientas de red, la petición enviada es `GET /tasks/<id>/toggle` con respuesta 404.

## Alcance

Incluido:

- Cambiar la llamada `fetch` de `public/assets/app.js` para enviar el formulario como lo haría el navegador: `POST` a `form.action` con el cuerpo del propio formulario (`authenticity_token` y `_method=PUT`) codificado como `application/x-www-form-urlencoded`.
- Si la respuesta no es correcta (`!response.ok`) o la petición falla, recargar la página para mostrar el estado real guardado.
- Un test de regresión en Go que compruebe que el JavaScript servido envía la alternancia por POST con los datos del formulario.

Excluido:

- Cambios en rutas, acciones, modelo, plantillas o CSS. El servidor ya funciona correctamente.
- Añadir un runner de tests de JavaScript o de navegador.
- Cambiar el comportamiento de confirmación (`data-confirm`) u otros formularios.
- Mensajes de error visibles específicos para fallos de red.

## Archivos relevantes

| Ruta | Responsabilidad |
|---|---|
| `public/assets/app.js` | Manejador de `submit` que actualiza la UI y envía la alternancia (aquí está el fallo) |
| `templates/tasks/index.plush.html` | Formulario `data-task-toggle` con `authenticity_token` y `_method=PUT` (sin cambios) |
| `actions/app.go` | Ruta `PUT /tasks/{task_id}/toggle` (sin cambios) |
| `actions/tasks.go` | `TasksToggle` alterna y guarda `completed`, responde 303 a `/` (sin cambios) |
| `actions/tasks_test.go` | Tests de acciones; aquí se añade el test de regresión |
| `public/embed.go` | Sirve `public/` (incluido `assets/app.js`) embebido |

## Implementación paso a paso

1. Confirmar la reproducción manual descrita arriba.
2. En `public/assets/app.js`, sustituir `fetch(form.action);` por:

   ```js
   fetch(form.action, {
     method: "POST",
     body: new URLSearchParams(new FormData(form)),
   })
     .then(function (response) {
       if (!response.ok) {
         throw new Error("No se pudo guardar la tarea");
       }
     })
     .catch(function () {
       window.location.reload();
     });
   ```

   `URLSearchParams` hace que el cuerpo se envíe como `application/x-www-form-urlencoded`, igual que el envío nativo del formulario, de modo que `MethodOverride` lee `_method=PUT` y el middleware CSRF lee `authenticity_token`. `fetch` sigue la redirección 303 a `/` con un GET; esa respuesta es 200 y se ignora. Mantener el estilo del archivo (`var`, funciones anónimas, sin funciones flecha) y añadir un comentario breve sobre por qué se envía el formulario completo.
3. Añadir el test de regresión (siguiente sección).
4. Ejecutar las validaciones.

## Tests y validación

Test de regresión en `actions/tasks_test.go`:

- `Test_AppJS_TogglePersistsViaFormPost`: `GET /assets/app.js` responde 200 y el cuerpo contiene `method: "POST"` y `new FormData(form)`, y **no** contiene `fetch(form.action);`. Antes del arreglo falla (el archivo contiene `fetch(form.action);` y no envía POST). Es un test estático: protege contra volver a la llamada GET, pero no ejecuta el JavaScript.

Los tests de servidor existentes (`Test_TasksToggle_*`) deben seguir pasando sin cambios.

Validación manual (necesaria, porque no hay tests de navegador):

1. Repetir la reproducción: completar una tarea, recargar → sigue completada. En la red se ve `POST /tasks/<id>/toggle` → 303 y luego `GET /` → 200.
2. Reabrir esa tarea, recargar → sigue pendiente.
3. Comprobar que la UI cambia al instante y que `aria-pressed` y la etiqueta del botón se actualizan.

Comandos (los tests necesitan PostgreSQL y las migraciones de test, como en `.github/workflows/ci.yaml`):

```bash
gofmt -l .
go vet ./...
soda migrate -e test
go test -p 1 ./...
go build ./...
```

## Riesgos y recuperación

- **Test estático frágil**: depende del texto exacto del JavaScript. Si se reescribe `app.js`, habrá que adaptarlo; no sustituye a una prueba en navegador.
- **Recarga en caso de error**: si el servidor falla, la página se recarga y el usuario ve el estado real sin un mensaje explicativo. Es preferible a mostrar un estado falso; un mensaje de error queda fuera de alcance.
- **Peticiones repetidas rápidas**: varios clics seguidos envían varias alternancias en orden de llegada; el servidor alterna cada vez, igual que con el envío nativo. No se introduce bloqueo.
- **Recuperación**: revertir el cambio de `app.js` devuelve el comportamiento anterior; no hay migraciones ni cambios de datos.
