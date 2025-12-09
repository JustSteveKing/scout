# Scout

`Monitor the health of your services from the terminal.`

Scout is a terminal-based dashboard for monitoring multiple services and APIs. Configure your endpoints, authentication, and health checks in a single config file, then launch Scout to see real-time status across all your services.

Perfect for keeping an eye on staging environments, microservices, or any APIs you depend on during development.

## Installation

```bash
go install github.com/juststeveking/scout@latest
```

## Usage

Initialize the configuration:

```bash
scout init
```

Add a service to monitor:

```bash
scout service:add
```

Run the monitor:

```bash
scout
```

## Configuration

Configuration is stored in `~/.config/scout/config.yml` (or equivalent on your OS).

## License

MIT
