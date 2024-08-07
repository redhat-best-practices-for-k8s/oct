# oct - Offline Catalog Tool for Red Hat's certified containerized artifacts
OCT is a containerized application that retrieves the latest certified artifacts (operators, containers and helm charts) from the [Red Hat's online catalog](https://catalog.redhat.com/api/containers/v1/ui/) to be used by RH's certification tools like [TNF](https://github.com/redhat-best-practices-for-k8s/certsuite) or similar.

# Important
This is a Work in Progress PoC/demo project, not a complete/GA nor a ready to use tool. Most of the code was copied from the [TNF repo](https://github.com/redhat-best-practices-for-k8s/certsuite) in order to get a running code quickly.

# Motivation
## Completely disconnected environments issue

Typically, partners run the TNF container in a "bastion" host or whatever Linux machine with access to the OCP nodes. In case that machine does not have internet access for security or any other reasons, TNF will run in a fully/completely disconnected environment. That's not a problem for TNF, except for one test suite: `affiliated-certification`. The test cases on that test suite will check the certification status of the CNF's containers, operators and/or helm charts releases. To do so, it relies on two mechanisms: first, these test cases will try to reach the Red Hat's online catalog, which is just a regular HTTP rest service that can be accessed from anywhere, anytime. In case this service is not reachable, which will happen in a completely disconnected environment, the test case falls back to an "offline" check mechanism, which involves querying the TNF's embedded catalog. So, if the TNF release is too old, those test cases could fail, as the RH's catalog for certified artifacts is continuously updated, with a lot of new entries being added, modified or removed every day.

Note: Only partner certified images are stored in the offline database. If Redhat images are checked against the offline database, they will show up as not certified. The online database includes both Partner and Redhat images.

If a partner is using the latest official TNF release, let's say v4.1, it will come with an embedded offline catalog that was downloaded and embedded at the same moment the v4.1 version was released. For each day that passes, that offline catalog will be more and more outdated, but the partners will be using that v4.1 until v4.2 comes out, which could take months. If they upgrade their CNFs with new operators/containers that have been added recently to the online catalog, the TNF certification test cases will fail.

The OCT container image will help here, as it's updated four times a day. The workflow for partners that want to run TNF in fully disconnected environments would be the following:
1. In a separate server, with internet access, download the latest OCT image container.
2. Copy that image into the bastion host, or the machine where TNF will run.
3. Run the OCT container in dump-only mode, which will create a folder with the internal catalog files.
4. Run TNF, indicating where it should find the offline catalog files, in case it needs them, which will surely happen.
# Usage

TNF's fetch CLI tool code was copied here, so the syntax is the same, but it's hidden to the user, as it only needs to provide the output folder where the DB will be copied. Two things can be done with the container: (1) getting the current db stored in the container or (2) run the container app to parse the online catalog to dump the latest version.
1. Create local container image or download the latest container image from quay.io/testnetworkfunction/oct:latest
    - To create a local container image from a local checkout folder, use this docker command:
      ```
      docker build -t quay.io/greyerof/oct_local:test --build-arg OCT_LOCAL_FOLDER=. --no-cache -f Dockerfile.local .
      ```
      `OCT_LOCAL_FOLDER` should point to the folder where the oct source code/checkout is.
2. Get the current container's offline db.
    - Create the local folder where the db will be copied by the container.
      ```
      $ mkdir db
      ```
    - Run the container, using adding the `--env OCT_DUMP_ONLY=true` params to the docker run command. The path to the local folder must be passed with the `-v`flag. Also, the user id and groups need to be used so the files are created with that user.
      ```
      $ docker run -v /full/path/to/db:/tmp/dump:Z --user $(id -u):$(id -g) --env OCT_DUMP_ONLY=true quay.io/greyerof/oct:latest
      OCT: Dumping current DB to /tmp/dump
      $ tree db
      db
      └── data
          ├── archive.json
          ├── containers
          │   └── containers.db
          ├── helm
          │   └── helm.db
          └── operators
              ├── operator_catalog_page_0_500.db
              ├── operator_catalog_page_1_500.db
              ├── operator_catalog_page_2_500.db
              ├── operator_catalog_page_3_500.db
              ├── operator_catalog_page_4_500.db
              ├── operator_catalog_page_5_500.db
              ├── operator_catalog_page_6_500.db
              └── operator_catalog_page_7_460.db

      ```
      This command tells the container app to to nothing but copying the internal db into the destination folder.

3. Get the latest (online) db.
    - Create the local folder where the db will be copied by the container. Same params as in (1) but without the env var.
      ```
      $ mkdir db
      ```
    - Run the container without any env var.
      ```
      $ docker run -v /full/path/to/db:/tmp/dump:Z --user $(id -u):$(id -g) quay.io/greyerof/oct:latest
      time="2022-07-15T10:11:57Z" level=info msg="{23196 3960 0}"
      time="2022-07-15T10:11:58Z" level=info msg="we should fetch new data3961 3960"
      ...
      $ tree db
      db
      └── data
          ├── archive.json
          ├── containers
          │   └── containers.db
          ├── helm
          │   └── helm.db
          └── operators
              ├── operator_catalog_page_0_500.db
              ├── operator_catalog_page_1_500.db
              ├── operator_catalog_page_2_500.db
              ├── operator_catalog_page_3_500.db
              ├── operator_catalog_page_4_500.db
              ├── operator_catalog_page_5_500.db
              ├── operator_catalog_page_6_500.db
              └── operator_catalog_page_7_460.db

      ```

# How it works
The `oct` container has a script [run.sh](https://github.com/redhat-best-practices-for-k8s/oct/blob/main/scripts/run.sh) that will run the TNF's `fetch` command. If it succeeds, the db files are copied into the container's `/tmp/dump` folder. The `OCT_DUMP_ONLY` env var is used to bypass the call to `fetch` so the current content of the container's db is copied into /tmp/dump.

The `fetch` tool was updated in this repo to retrieve the operator and container pages in a concurrent way, using a go routine for each http get. I did this modification to speed up the dev tests, as the original `fetch` implementation to the the sequantial retrieval of catalog pages was too slow (RH's online catalog takes a lot of time for each response). Do not pay too much attention to the current implementation, as the original `fetch` tool's code was copied some weeks ago, and it was modified later by some PRs in the TNF's repo.
# Issues/Caveats/Warnings
The current TNF's `fetch` tool (used in `oct`) works like this:
1. First it calls the catalog api to get the number of artifacts.
2. In case this number don't match the last saved ones, it starts asking for each of the catalog pages, with a maximum size of 500 entries per page. This has two problems:
   - In case the catalog is being updated at that very moment, and a new page has been added, that page probably won't be retrieved. Not to mention the case where they remove some entry from a page already retrieved...
   - If they have removed one entry but also added a new one, the final number is the same, making the `fetch` tool to consider its DB does not need to be updated, which is wrong.
3. The current workflow to create a new `oct` container has two issues:
   - The catalog API to get the certified containers is not working quite well lately, so the workflow fails too many times.
   - The workflow uses the `fetch`tool directly, but the repo's db index is empty, so it will always create a new container, no matter if it contains exactly the same files as the latest one. This can easily be solved by improving the workflow to dump the latest container's db and comparing it with the new one obtained by the `fetch` tool. Whatever solution is implemented, it should avoid uploading the db files into this repo.
4. The DB format of the `oct` tool is coupled to the one that TNF expects, where it should be the other way around (TNF depending on oct's repo) or both repos importing a third repo with the API/scheme only (as TNF does with the [claim file format](https://github.com/redhat-best-practices-for-k8s/certsuite-claim)).

In my opinion, as these certification processes have begun to play important roles in the Red Hat's Ecosystem, this kind of tools shouldn't be needed, as the RH's online catalog could easily provide (a) its own container with this information or (b) a way to download a db.tar.gz with it.
