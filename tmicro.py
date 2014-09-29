#!/usr/bin/env python
import pika
import json
import requests
import time

import SOAPpy
from SOAPpy import WSDL
from SOAPpy import SOAPProxy 


# Creates a connection to the RabbitMQ broker running on localhost
connection = pika.BlockingConnection(pika.ConnectionParameters(
                                     host='localhost'))

# Gets a channel to use for communicating with the broker
channel = connection.channel()

# Declares a new queue within the broker
result = channel.queue_declare(exclusive=True)

# Finds the auto-generated queue name to use when binding the queue
# to the exchange
queue_name = result.method.queue
print 'Created Queue: ' + queue_name

# Binds the queue to the cloudstack-events
# exchange, with a wildcard routing key.
# The wildcard key will cause all messages
# sent to the exchange to be published to
# your new queue.
channel.queue_bind(exchange='cloudstack-events',
                   queue=queue_name,
                   routing_key = '#')

print ' [*] Waiting for logs. To exit press CTRL+C'

# A simple callback method that will print
# the routing_key and message body for any
# message that it receives.
def callback(ch, method, properties, body):
    #print " [x] %r:%r" % (method.routing_key, body,)
    try:
        message = json.loads(body)
        if "id" in message and message["id"] != None:
            if "event" in message and message["event"] == "VM.CREATE" and message["id"] != None:
                print "VM create event", message
                vmId = message["id"]
                apiUrl = "http://localhost:8096/client/api?command=listVirtualMachines&details=nics&id=%s&response=json" % vmId
                # Try three times
                for i in range(3):
                    try:
                        response = requests.get(apiUrl).json()
                        print "response for api call:", response
                        ipaddress = response['listvirtualmachinesresponse']['virtualmachine'][0]['nic'][0]['ipaddress']
                        break
                    except Exception:
                        pass
                    time.sleep(2)

                print "Trying to register host on Deep security:", ipaddress

                if ipaddress != None:
                    print "Authenticating with SOAP server"
                    wsdlFile = "tmicro.wsdl"
                    server = WSDL.Proxy(wsdlFile)
                    #server.soapproxy.config.dumpSOAPIn = 1
                    #server.soapproxy.config.dumpSOAPOut = 1
                    sID = server.authenticate('Admin1!', 'Password1!')
                    hostTransport = SOAPpy.structType()
                    hostTransport._addItem('name', str(ipaddress))
                    hostTransport._addItem('displayName', message["id"])
                    result = server.hostCreate(host=hostTransport, sID=sID)
                    print "Deep Security added for host:", result.name
                    server.endSession(sID)


            if "resource" in message and message["resource"] == "VirtualMachine" and "new-state" in message and message["new-state"] == "Expunging":
                vmId = message["id"]
                wsdlFile = "tmicro.wsdl"
                server = WSDL.Proxy(wsdlFile)
                #server.soapproxy.config.dumpSOAPIn = 1
                #server.soapproxy.config.dumpSOAPOut = 1
                sID = server.authenticate('Admin1!', 'Password1!')
                hosts = server.hostRetrieveAll(sID)
                if not isinstance(hosts, list):
                    hosts = [hosts]

                for host in hosts:
                    if host.displayName == vmId:
                        print "Removing deep security on host", host.displayName, host.name, host.ID
                        result = server.hostDelete([int(host.ID)], sID)
                        print "Host was removed", host.displayName, host.name, host.ID
                        break
                server.endSession(sID)


    except Exception, e:
        print "exception:", e


# Tell the channel to use the callback
channel.basic_consume(callback,
                      queue=queue_name,
                      no_ack=True)

# And start consuming events!
channel.start_consuming()
