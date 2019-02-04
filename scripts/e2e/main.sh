input:  secrets/enviroment variables

pipeline:

1) create management cluster
2) apply clusterapi CRD
3) validate management cluster

4) create jobs at management cluster
   
   make sure the first basic job passed before deploy all jobs.
   make sure clean up all resources after each job even when job failed.

