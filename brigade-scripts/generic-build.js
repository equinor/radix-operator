const { events, Job, Group } = require('brigadier')

events.on("exec", (e, p) => {
    let buildId = e.revision.commit
    let appName = p.name.substring(p.name.lastIndexOf("/") + 1);
    let updateConfig = applyRadixConfig(appName)
    updateConfig.run().then(res => {
        runBuild(res.data, buildId, p)
    });
});

function runBuild(config, buildId, project) {    
    let components = config.spec.components;
    let environments = config.spec.environments
    let buildPipeline = new Group()
    let deployPipeline = new Group()
    components.forEach(component => {
        let name = component.name
        let imageName = `${project.secrets.DOCKER_REGISTRY}/${config.metadata.name}-${name}:${buildId}`

        let buildJob = buildComponent(name, imageName, component.src, project);
        buildPipeline.add(buildJob);
        environments.forEach(env => {
            let deployJob = deployComponent(name, imageName, `${config.metadata.name}-${buildId}`, env, project);
            deployPipeline.add(deployJob);
        })
    });

    buildPipeline.runAll().then(() => deployPipeline.runAll());
}

function applyRadixConfig(appName) {
    let job = new Job("config", "radixdev.azurecr.io/rx:6c835aa");
    job.imagePullSecrets = ["radixdev-docker"]
    job.serviceAccount = "radix-deploy"
    job.tasks = [
        "cd /src",
        `kubectl apply -f radixconfig.yaml -n${appName}-app > /dev/null`,
        "sleep 5",
        `kubectl get ra ${appName} -n${appName}-app -ojson`
    ];

    return job;
}

function deployComponent(name, imageName, buildId, environment, project){
    let job = new Job("deploy-"+buildId, "radixdev.azurecr.io/rx:6c835aa");
    job.imagePullSecrets = ["radixdev-docker"]
    job.serviceAccount = "radix-deploy"
    job.tasks = [
        "cd /src",
        `rx deploy create ${buildId} -i ${name}=${imageName}`
    ];
     
    return job;
}

function buildComponent(name, imageName, src, project) {
    console.log("building: " + imageName)

    let driver = project.secrets.DOCKER_DRIVER || "overlay"

    let docker = new Job("build-" + name, "docker:stable-dind")
    docker.privileged = true;
    docker.timeout = 9000000
    docker.env = {
        DOCKER_DRIVER: driver
    }
    docker.tasks = [
        "dockerd-entrypoint.sh &",
        "sleep 20",
        "cd /src/" + src,
        "docker build -t " + imageName + " ."
    ];

    if (project.secrets.DOCKER_USER) {
        docker.env.DOCKER_USER = project.secrets.DOCKER_USER
        docker.env.DOCKER_PASS = project.secrets.DOCKER_PASS
        docker.env.DOCKER_REGISTRY = project.secrets.DOCKER_REGISTRY
        docker.tasks.push("docker login -u $DOCKER_USER -p $DOCKER_PASS $DOCKER_REGISTRY")
        docker.tasks.push("docker push " + imageName)
    } else {
        console.log("skipping push. DOCKER_USER is not set.")
    }

    return docker;
}