<!-- markdownlint-disable MD041 -->
`subctl verify` has been enhanced to select nodes labeled with `test.submariner.io/non-gateway-node=true` as non-Gateway
nodes while scheduling the test pods. If none of the nodes have the label `test.submariner.io/non-gateway-node=true`, the
test framework falls back to the existing approach of randomly selecting a non-Gateway node.
