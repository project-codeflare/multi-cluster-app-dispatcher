import kopf
import kubernetes
import yaml

@kopf.on.create('ibm.com', 'v1beta1', 'resourceplans')
def create_fn(body, spec, **kwargs):
    # Get info from Database object
    name = body['metadata']['name']
    namespace = body['metadata']['namespace']
    type = spec['type']

    # Make sure type is provided
    #if not type:
    #    raise kopf.HandlerFatalError(f"Type must be set. Got {type}.")

    # Pod template
    pod = {'apiVersion': 'v1', 'metadata': {'name' : name, 'labels': {'app': 'db'}}}

    # Service template
    svc = {'apiVersion': 'v1', 'metadata': {'name' : name}, 'spec': { 'selector': {'app': 'db'}, 'type': 'NodePort'}}

    # Update templates based on Database specification

    #if type == 'mongo':
    #  image = 'mongo:4.0'
    #  port = 27017
    #  pod['spec'] = { 'containers': [ { 'image': image, 'name': type } ]}
    #  svc['spec']['ports'] = [{ 'port': port, 'targetPort': port}]
    #if type == 'mysql':
    image = 'mysql:8.0'
    port = 3306
    pod['spec'] = { 'containers': [ { 'image': image, 'name': type, 'env': [ { 'name': 'MYSQL_ROOT_PASSWORD', 'value': 'my_passwd' } ] } ]}
    svc['spec']['ports'] = [{ 'port': port, 'targetPort': port}]

    # Make the Pod and Service the children of the Database object
    kopf.adopt(pod, owner=body)
    kopf.adopt(svc, owner=body)

    # Object used to communicate with the API Server
    api = kubernetes.client.CoreV1Api()

    # Create Pod
    obj = api.create_namespaced_pod(namespace, pod)
    print(f"Pod {obj.metadata.name} created")

    # Create Service
    obj = api.create_namespaced_service(namespace, svc)
    print(f"NodePort Service {obj.metadata.name} created, exposing on port {obj.spec.ports[0].node_port}")

    # Update status
    msg = f"Pod and Service created by Resource Plan {name}"
    return {'message': msg}

@kopf.on.delete('ibm.com', 'v1beta1', 'resourceplans')
def delete(body, **kwargs):
    msg = f"Resource Plan {body['metadata']['name']} and its Pod / Service children deleted"
    return {'message': msg}
