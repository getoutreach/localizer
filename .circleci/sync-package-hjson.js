const { execSync } = require("child_process");
const fs = require("fs");
const hjson = require("hjson");
const os = require("os");
const path = require("path");

function execPassthru(cmd) {
  console.log(exec(cmd));
}

function exec(cmd) {
  return execSync(cmd).toString().trim();
}

const relativeNodeClientPath = path.join("api", "clients", "node");
const nodeClientPath = path.resolve(__dirname, "..", relativeNodeClientPath);
if (!fs.existsSync(nodeClientPath)) {
  console.log("No Node.js gRPC client found, exiting");
  process.exit(0);
}

const added = exec(
  "git show --pretty=format: --unified=0 HEAD -- api/clients/node/package.json"
)
  .split("\n")
  .find((line) => line.startsWith("+ "));

if (added === undefined) {
  console.log("Could not find a line that was added");
  process.exit();
}

console.log("added", added);

const match = /^\+\s+"(?<name>.*)": "(?<version>.*)",?/.exec(added);
if (match === null) {
  console.log("No changed dependency found, exiting");
  process.exit();
} else {
  const hjsonPath = path.join(nodeClientPath, "package.hjson");
  const source = hjson.rt.parse(fs.readFileSync(hjsonPath).toString());

  let foundDep = false;
  for (const depType of ["dependencies", "devDependencies"]) {
    for (const [depName, depVersion] of Object.entries(source[depType])) {
      if (depName === match.groups.name) {
        console.log(
          `Changing ${depName} from ${depVersion} to ${match.groups.version}`
        );
        source[depType][depName] = match.groups.version;
        fs.writeFileSync(
          hjsonPath,
          hjson.rt.stringify(source, {
            bracesSameLine: true,
            quotes: "all",
            separator: true,
          }) + os.EOL
        );

        console.log("Committing to git");
        const netrc = path.join(os.homedir(), ".netrc");
        if (process.env.OUTREACH_GITHUB_TOKEN) {
          execPassthru('git config user.name "Outreach CI"');
          execPassthru(
            "git config user.email outreach-ci@users.noreply.github.com"
          );
          execPassthru(`git add "${hjsonPath}"`);
          execPassthru(
            `git commit -m "chore: sync ${path.join(
              relativeNodeClientPath,
              "package.hjson"
            )}"`
          );
          fs.writeFileSync(
            netrc,
            `machine github.com login outreach-ci password ${process.env.OUTREACH_GITHUB_TOKEN}`
          );
          fs.chmodSync(netrc, 0o600);
        } else {
          console.log("No GitHub token found");
          process.exit();
        }
        const branchName = process.env.CIRCLE_BRANCH;
        if (!branchName) {
          console.error(
            `Unknown source branch name: "${JSON.stringify(branchName)}"`
          );
          process.exit(1);
        }
        console.log(`Pushing commit to ${branchName}`);
        execPassthru(`git push origin HEAD:${branchName}`);
        foundDep = true;
        break;
      }
    }

    if (foundDep) {
      break;
    }
  }

  if (!foundDep) {
    console.error("Could not find corresponding dependency in package.hjson");
  }
}
