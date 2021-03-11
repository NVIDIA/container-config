
### Running toolkit tests locally

```bash
make build-ubuntu18.04
````

To run only the toolkit tests:
```bash
export TEST_CASES=toolkit
make test-ubuntu18.04
```

To check the generated files in `shared-ubuntu18.04` (i.e. skipping cleanup)
```bash
export CLEANUP=false
make test-ubuntu18.04
```


