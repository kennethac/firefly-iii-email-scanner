# Mattermost Package

## Generated API client

I downloaded the [openapi.json](./openapi.json) file from [the Mattermost api docs](https://api.mattermost.com/). The oapi-codegen package failed to generate the client code until I removed all of the `format: int64` tags, which apparently it doesn't handle.

Once I had done _that_ there were still a bunch of routes that had missing or bad parameter specifications. I just removed a few routes and corrected (maybe) a few others until it build.