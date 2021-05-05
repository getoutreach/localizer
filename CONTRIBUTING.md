# localizer

<!--- Block(custom) -->
<!--
We expect CONTRIBUTING.md to look mostly identical for all bootstrap services.

If your service requires special instructions for developers, you can place
those instructions in this block. If your service isn't special, it's safe to
leave this comment here as-is.

If the text you are about to add here applies to many or all bootstrap services,
consider adding it to the bootstrap template instead.
-->
<!--- EndBlock(custom) -->

The following sections of CONTRIBUTING.md were generated with
[bootstrap](https://github.com/getoutreach/bootstrap) and are common to all
bootstrap services.

## Dependencies

Make sure you've followed the [Launch Plan](https://outreach-io.atlassian.net/wiki/spaces/EN/pages/695698940/Launch+Plan).
[Set up bootstrap](https://outreach-io.atlassian.net/wiki/spaces/EN/pages/701596137/Services+Checklist) if you're planning on updating bootstrap files.

<!--- Block(devDependencies) -->
<!--- EndBlock(devDependencies) -->

## Building and Testing

<!--- Block(buildCustom) -->
<!--- EndBlock(buildCustom) -->

### Building (Locally)

To produce binaries in the `./bin/` folder, run `make build`.

### Unit Testing

You can run the tests with:

```bash
make test
```


## Releasing

Making releases for this repository follows the process in the [Bootstrap](https://github.com/getoutreach/bootstrap/tree/master/README.md#semver) documentation.
