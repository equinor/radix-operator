const { events, Job, Group } = require('brigadier')

events.on("exec", (e, p) => {
    var app = p.secrets.app.replace(new RegExp("'", 'g'), "\"")
    var components = JSON.parse(app);
    components.forEach(component => {
        console.log("Component: " + component.name);
        console.log("Building from: " + component.src)
        console.log("Public: " + component.public)
    });
});

events.on("push", (e, p) => {
    let buildId = e.revision.commit
    let imageName = "staas.azurecr.io/prodcom-pilot:" + buildId
    console.log("building: " + imageName)

    const build = new Job("build-image", "docker:18.02")
    build.env = {
        DOCKER_HOST: "tcp://docker:2375"
    }
    const buildCmd = "docker build -t " + imageName + " -f prodcom.api/Dockerfile ."
    build.tasks = [
        "cd /src",
        buildCmd
    ]
    if (project.secrets.DOCKER_USER) {
        build.env.DOCKER_USER = project.secrets.DOCKER_USER
        build.env.DOCKER_PASS = project.secrets.DOCKER_PASS
        build.env.DOCKER_REGISTRY = project.secrets.DOCKER_REGISTRY
        build.tasks.push("docker login -u $DOCKER_USER -p $DOCKER_PASS $DOCKER_REGISTRY")
        build.tasks.push("docker push " + imageName)
    } else {
        console.log("skipping push. DOCKER_USER is not set.")
    }

    const deploy = new Job("deploy", "frodehus/kubectl:latest")
    deploy.serviceAccount="jenkins-jenkins";
    const regex = /\//gm;
    const subst = `\\/`;
    var deployImage = imageName.replace(regex, subst);
    deploy.tasks = [
        "cd /src",
        "sed -i 's/IMAGE_NAME/" + deployImage + "/' deploy.yaml",
        "kubectl apply -f deploy.yaml"
    ]

    var pipeline = new Group();
    pipeline.add(build);
    pipeline.add(deploy);

    pipeline.runEach();

});