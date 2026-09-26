Sheidan is a framework for building modern web apps in golang.

It's open-source and designed for building to an insane scale insanely quick. Dream a thing and start building, refactoring as you go.

The full stack is written in golang, with front-end components transpiled to JavaScript in a Vue-like MVVC progressive web app pattern.

# @todo Rewrite this README. Design notes:

- Decoupled front-end built around [Templ](https://github.com/a-h/templ) templates with components transpiled with (GopherJS)[https://github.com/gopherjs/gopherjs].
  - Render idempotent components server or client-side.
  - Clean division between Model (Go + GORM Backend), ViewModel (Data-binding with Controllers, Routing, Mapping), and View (Templates & Observation Rendering Logic).
- Built on [Gin](https://github.com/gin-gonic/gin).
- Treats [Turso](https://github.com/tursodatabase/turso) as a first-class database.
- Uses [GORM](https://gorm.io/) SQLite driver for database interactions.
- Includes plug-ins for [DCDC](https://github.com/cybtachyon/DCDC) as an artisan CLI for all tasks.
  - Storybook-like Templ component development.
  - Seeding sample data for testing and validation.
  - Common tasks like database migrations.
  - Live reload via [Go Air](https://github.com/air-verse/air)
- Pre-built DCDC starter kits for everything from custom enterprise management apps to homelab tooling.

## References
- https://github.com/a-h/templ/tree/main/examples/integration-gin
