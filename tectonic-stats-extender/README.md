# tectonic-stats-extender

[![Container Repository on Quay](https://quay.io/repository/coreos/tectonic-stats-extender/status?token=17a1091d-1fe9-4da2-af92-416b8a0e5f54 "Container Repository on Quay")](https://quay.io/repository/coreos/tectonic-stats-extender)

tectonic-stats-extender is a basic implementation of a sidecar for [Spartakus](https://github.com/kubernetes-incubator/spartakus) that is customized to report information on a Tectonic license. The extender is in charge of writing extra data you may want to report to a file that can be read by Spartakus.

## Building

The tectonic-stats-extender binary can be built with:

```sh
make build
```

## Running

The extender binary can be run like:

```sh
./bin/amd64/tectonic-stats-extender --license=/path/to/tectonic/license --output=/path/to/output/file --extensions=extra:data --extensions=to:report
```

The default file generation interval is 1 hour, though this can be customized with the `--period` flag.

## Testing

To test this binary with the example license provided in the tectonic-licensing repo, run the following command:

```sh
./bin/amd64/tectonic-stats-extender --output=extensions --license=vendor/github.com/coreos-inc/tectonic-licensing/license/test-license.txt --public-key=vendor/github.com/coreos-inc/tectonic-licensing/license/test-signing-key.pub
```

If this worked, you will see something like the following logged:

```sh
INFO[0000] started stats-extender
INFO[0000] successfully generated extensions
```

You should now see a file called `extensions` with the following contents:

```json
{"accountID":"ACC-FA720BE4-6C55-476A-812C-C4CA6862"}
```

To try setting some custom extensions using the `--extension` flag in addition to the license, try the following:

```sh
./bin/amd64/tectonic-stats-extender --output=extensions --license=vendor/github.com/coreos-inc/tectonic-licensing/license/test-license.txt --public-key=vendor/github.com/coreos-inc/tectonic-licensing/license/test-signing-key.pub --extension=newKey:newValue
```

If this worked, you will see something like the following logged:

```sh
INFO[0000] started stats-extender
INFO[0000] successfully generated extensions
```

You should see the following content in the `extensions` file:

```json
{"accountID":"ACC-FA720BE4-6C55-476A-812C-C4CA6862","newKey":"newValue"}
```

## Running the Test Container

This repository also provides a test container to validate the entire Tectonic statistics-gathering pipeline. The full suite is a fairly comprehensive integration test that ensures that metrics from a given Tectonic cluster make it to the ultimate destination that is the Tectonic statistics BigQuery table.

To compile the test binary, run:

```sh
make test
```

The test binary requires the following input in order to run the complete test suite:

* the `KUBECONFIG` environment variable must be set to the file path of a configuration for the target Tectonic cluster;
* the flag `--bigqueryspec` must be provided and set to the spec for the target BigQuery table, e.g. `--bigqueryspec bigquery://project.dataset.table`; and
* the `GOOGLE_APPLICATION_CREDENTIALS` environment variable must be set to the file path of BigQuery credentials in order to authenticate.

*Note*: if no `--bigqueryspec` flag is provided then a partial integration test suite will be run; this test suite only validates that statistics were successfully sent and *not* whether they were successfully added to the BigQuery table.

To execute the test binary, run:

```sh
export KUBECONFIG=/path/to/kubeconfig
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/bigquery/credentials
./bin/amd64/tectonic-stats-test --bigqueryspec bigquery://project.dataset.table
```

If the program exits with a non-zero exit code then one or more of the tests failed.
