<!-- markdownlint-disable MD041 -->
Support for simulated "air-gapped" environments has been added to kind clusters.
The air-gap is simulated by blocking outgoing traffic to any public subnet on the cluster nodes, effectively isolating the host network.
To use, deploy with `USING=air-gap` or `AIR_GAPPED=true`.
