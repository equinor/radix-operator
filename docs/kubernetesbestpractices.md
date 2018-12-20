BEST PRACTICES RESOURCE LIMITS AND QUOTAS:
- Keep cpu requests below 1 core and scale horizontally. 
    - if not heavy cpu intencive 
    - 1000m = 1CPU
- cpu limits
    - trottle if above
    - 500m will give posibility to go up to 1CPU = 1000CPU
- memory limits
    - if above, will kill process 64 MiB = 2^26 bytes


ResourceQuotas:
- request.cpu / memory
    - max request all containers combined can have in namespace

Overcommitment:
- cpu -> trottle applications
- memory 
    - kubernetes will look for pods using more resources then requested
        - if no request / limit -> prime candidate for termination
        - if over request under limit -> prime candidate for termination
        - pod priorty (should be used for our component?)
        - of equal priority -> terminate the one that has gone the most over it request

Commands:
kubectl get nodes --no-headers | awk '{print $1}' | xargs -I {} sh -c 'echo {}; kubectl describe node {} | grep Allocated -A 5 | grep -ve Event -ve Allocated -ve percent -ve -- ; echo'

kubectl top pod --all-namespaces


DOCKER RESOURCES
building small images - how (small base images & multistage/builder pattern) and why (performance & security)
- https://www.youtube.com/watch?v=wGz_cbtCiEA&list=PLIivdWyY5sqL3xfXz5xJvwzFW_tlQB_GB


