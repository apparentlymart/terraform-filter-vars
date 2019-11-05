# terraform-filter-vars

`terraform-filter-vars` is a helper tool for wrangling one or more "superset"
`.tfvars` files into a single `.tfvars` file containing only the definitions
relevant to a particular root Terraform module.

In systems that have common conventions around running Terraform such that
some variables are defined in the same way across many configurations, it can
be convenient to produce one or more "superset" variables files that contain
values for all of those variables, even though not all of the configurations
will use all of them.

As an aid to those who might've made typos in their `.tfvars` files, Terraform
generates warnings for definitions of undeclared variables, which can be
off-putting for those using "superset" variables files.

`terraform-filter-vars`, then, allows preprocessing of these superset files
to filter down to only the variables that are declared for a particular module,
so that Terraform won't generate those warnings.

There are
[pre-compiled releases of this program](https://github.com/apparentlymart/terraform-filter-vars/releases)
targeting the same platforms that Terraform itself targets.

## Example

In the `example` subdirectory of this program's repository are some `.tfvars`
files with the following contents:


```
# a.tfvars

foo = "foo from a.tfvars"

# Bar is blah blah blah
bar = "bar from a.tfvars"
```

```
# b.tfvars

# a.tfvars defines this one too, but if b.tfvars is later in the arguments list
# then it will "win" and override it.
bar = "bar from b.tfvars"

baz = "baz from b.tfvars"

# Not used by the example module, so will be filtered out
boop = "boop from b.tfvars"
```

There is also an example Terraform module in `./example/module` that has
declarations only for variable names `foo`, `bar`, and `baz`.

We can run `terraform-filter-vars` against that module and these example
`.tfvars` files to produce a merged and filtered result:

```
$ terraform-filter-vars ./example/module ./example/a.tfvars ./example/b.tfvars
# a.tfvars defines this one too, but if b.tfvars is later in the arguments list
# then it will "win" and override it.
bar = "bar from b.tfvars"
baz = "baz from b.tfvars"
foo = "foo from a.tfvars"
```

Notice that `boop` is not defined in the result at all, and that the definiton
of `bar` was taken from `b.tfvars` in preference to `a.tfvars` because it
appeared later in the command line arguments.

We can write that file to disk using the `-out` option or using shell I/O
redirection:

```
$ terraform-filter-vars -out=filtered.tfvars ./example/module ./example/a.tfvars ./example/b.tfvars
```

If creating a temporary file isn't desirable, and if we are using a shell
that supports automatic FIFO-based I/O redirection, we can use such a
redirection to pass the result directly to Terraform. For example, in `bash`
from within the `example/module` directory:

```
$ terraform plan -var-file=<(terraform-filter-vars . ../a.tfvars ../b.tfvars)
```

This `<( ... )` syntax in bash creates a pipe on a new file descriptor,
launches `terraform-filter-vars` with its stdout connected to the write end of
that pipe, and then replaces that sequences with a path that can be opened to
connect to the read end of the pipe, thus causing Terraform to read the
result as a `.tfvars` file.
