const { events, Job, Group } = require('brigadier')

events.on("push", (e, p) => {
    let buildId = e.revision.commit

    let updateConfig = applyRadixConfig()
    updateConfig.run().then(res => {
        runBuild(res.data, buildId, p)
    });
});

function runBuild(config, buildId, project) {
    console.log(config)
    let components = config.spec.components;
    let buildPipeline = new Group()
    let deployPipeline = new Group()
    components.forEach(component => {
        let name = component.name
        let imageName = project.secrets.DOCKER_REGISTRY + "/" + repo + "-" + name + ":" + buildId

        let buildJob = buildComponent(name, imageName, component.src, project);
        buildPipeline.add(buildJob);

        let deployJob = deployComponent(name, imageName, project);
        deployPipeline.add(deployJob);
    });

    buildPipeline.runAll().then(() => deployPipeline.runAll());
}

function applyRadixConfig() {
    let job = new Job("config", "frodehus/kubectl:latest");
    job.serviceAccount = "radix-deploy"
    job.tasks = [
        "cd /src",
        "kubectl apply -f radixconfig.yaml -ncomplete-app > /dev/null",
        "sleep 5",
        "kubectl get ra complete -ncomplete-app -ojson"
    ];

    return job;
}

function deployComponent(name, imageName, project){
    let job = new Job("deploy-"+name, "frodehus/kubectl:latest");
    job.serviceAccount = "radix-deploy"
    job.tasks = [
        "cd /src",
        "kubectl apply -f asdf"
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