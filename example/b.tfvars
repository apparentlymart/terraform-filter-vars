# a.tfvars defines this one too, but if b.tfvars is later in the arguments list
# then it will "win" and override it.
bar = "bar from b.tfvars"

baz = "baz from b.tfvars"

# Not used by the example module, so will be filtered out
boop = "boop from b.tfvars"
