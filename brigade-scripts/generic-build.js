const { events, Job, Group } = require('brigadier')

events.on("exec", (e, p) => {
    // let buildId = e.revision.commit
    let buildId = "12e34"
    let app = p.secrets.app.replace(new RegExp("'", 'g'), "\"")
    let components = JSON.parse(app);

    let updateConfig = applyRadixConfig()
    updateConfig.run()

    let pipeline = new Group()
    components.forEach(component => {
        let name = component.name
        let buildJob = buildComponent(name, buildId, "complete", component.src, p);
        pipeline.add(buildJob);
    });

    pipeline.runAll();
});

function applyRadixConfig() {
    let job = new Job("config", "frodehus/kubectl:latest");
    job.serviceAccount = "radix-deploy"
    job.tasks = [
        "cd /src",
        "kubectl apply -f radixconfig.yaml"
    ];

    return job;
}

function buildComponent(name, buildId, repo, src, project) {
    let imageName = "staas.azurecr.io/" + repo + "-" + name + ":" + buildId
    console.log("building: " + imageName)

    let driver = project.secrets.DOCKER_DRIVER || "overlay"

    // Build and push a Docker image.
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

    return docker;
}