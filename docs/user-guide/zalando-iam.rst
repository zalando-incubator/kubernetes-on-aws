.. _zalando-iam-integration:

================================
Zalando Platform IAM Integration
================================

**Introductory Remark**: After the learnings of two years of STUPS/IAM integration, we will integrate a more advanced and simpler solution.
The integration will be Kubernetes native and will take complexity out of your application.
Instead of providing you with client ID, client secret, username and password that you then have to use to generate tokens regularly, we will provide you a simple way to directly obtain the ready-to-go tokens instead of fiddling with the credentials.
Technically speaking, this means you just need to read your current token from a text file in your filesystem and you are done - no complicated token libraries anymore.

The user flow for a new application to get OAuth credentials looks like:

* Register the new application in the `Kio`_ application registry via the `YOUR TURN frontend`_.
* Configure OAuth scopes in Mint via the "Access Control" in the `YOUR TURN frontend`_.
* Configure required OAuth credentials (tokens and/or clients) in Kubernetes via a new :ref:`credentials-spec` resource.

.. _credentials-spec:

Platform IAM Credentials
========================

The ``PlatformCredentialsSet`` resource allows application owners to declare needed OAuth credentials.

.. code-block:: yaml

    apiVersion: "zalando.org/v1"
    kind: PlatformCredentialsSet
    metadata:
       name: my-app-credentials
    spec:
       application: my-app # has to match with registered application in kio/yourturn
       tokens:
         full-access: # token name
           privileges: # privileges/scopes for the token.
             # All zalando-specific privileges start with namespace com.zalando, following pattern <namespace>::<privilege>
             # the privileges/scopes you define here should match those you define for your application in yourturn.
             - com.zalando::foobar.write
             - com.zalando::acme.full
         read-only: # token name
           privileges: # privileges/scopes for the token.
             - com.zalando::foobar.read
       clients:
         employee: # client name
           # the allowed grant type, see https://tools.ietf.org/html/rfc6749
           # options: authorization-code, implicit, resource-owner-password-credentials, client-credentials
           # (values directly reference RFC section titles)
           grant: authorization-code
           # the client's account realm
           # options: users, customers, services
           # ("services" realm should not be used for clients, use the "tokens" section instead!)
           realm: users
           # redirection URI as described in https://tools.ietf.org/html/rfc6749#section-2
           redirectUri: https://example.org/auth/callback

The declared credentials will automatically be provided as a secret with the
same name.

Following this example you would get a token called ``full-access`` with the
privileges ``com.zalando::foobar.write`` and ``com.zalando::acme.full``, a
token called ``read-only`` with privileges ``com.zalando::foobar.read`` and a
client named ``employee`` which uses ``authorization-code`` grant under realm
``users``.


Secrets
=======

Automatically generated secrets provide the declared OAuth credentials in the following form:

.. code-block:: yaml

    apiVersion: v1
    kind: Secret
    metadata:
      name: my-app-credentials
    type: Opaque
    data:
      full-access-token-type: Bearer
      full-access-token-secret: JwAbc123.. # JWT token
      read-only-token-type: Bearer
      read-only-token-secret: JwBcd456.. # JWT token
      employee-client-id: 67b86a55-61e6-4862-aa14-70fe7be788f4
      employee-client-secret: 5585942c-ce79-44e4-aac2-8af565b51d3e

The secret can conveniently be mounted to read the tokens and client credentials from a volume:

.. code-block:: yaml

    apiVersion: extensions/v1beta1
    kind: Deployment
    metadata:
      name: my-app
    spec:
      replicas: 3
      template:
        metadata:
          labels:
            application: my-app
        spec:
          containers:
          - name: my-app
            image: pierone.stups.zalan.do/myteam/my-app:cd53
            ports:
            - containerPort: 8080
            volumeMounts:
            - name: my-app-credentials
              mountPath: /meta/credentials
              readOnly: true
          volumes:
          - name: my-app-credentials
            secret:
              secretName: my-app-credentials

The application can now simply read the declared tokens from text files, i.e. even a simple Bash script suffices to use OAuth tokens:

.. code-block:: bash

    #!/bin/bash
    type=$(cat /meta/credentials/read-only-token-type)
    secret=$(cat /meta/credentials/read-only-token-secret)
    curl -H "Authorization: $type $secret" https://resource-server.example.org/protected

Either use one of the `supported token libraries`_ or implement the file read on your own.
How to read a token in different languages:

.. code-block:: python

    # Python
    with open('/meta/credentials/{}-token-secret'.format(token_name)) as fd:
        access_token = fd.read().strip()


.. code-block:: javascript

    // JavaScript (node.js)
    const accessToken = String(fs.readFileSync(`/meta/credentials/${tokenName}-token-secret`)).trim()

.. code-block:: java

    // Java
    String accessToken = new String(Files.readAllBytes(Paths.get("/meta/credentials/" + tokenName + "-token-secret"))).trim();

.. Note::

    Using the authorization type from the secret instead of hardcoding ``Bearer`` allows to transparently
    switch to HTTP Basic Auth in a different context (e.g. running an Open Source application in a non-Zalando environment).
    Users would simply need to provide an appropriate secret like:

    .. code-block:: yaml

        apiVersion: v1
        kind: Secret
        metadata:
          name: my-app-credentials
        type: Opaque
        data:
          full-access-token-type: Basic
          full-access-token-secret: dXNlcjpwYXNzCg== # base64 encoded user:pass
          read-only-token-type: Basic
          read-only-token-secret: dXNlcjpwYXNzCg== # base64 encoded user:pass


Problem Feedback
================

Providing the requested credentials (tokens, clients) may fail for various reasons:

* the ``PlatformCredentialsSet`` has syntactic errors
* the application (``application`` property) does not exist or is missing required configuration
* the application is not allowed to obtain the requested credentials (e.g. missing privileges)
* some other error occurred

All problems with credential distribution are written to the secret with the same name as the ``PlatformCredentialsSet``:

.. code-block:: yaml

    apiVersion: v1
    kind: Secret
    metadata:
      name: my-app-credentials
      annotations:
        zalando.org/problems: |
          - type: https://credentials-provider.example.org/not-enough-privileges
            title: Forbidden: Not enough privileges
            status: 403
            instance: tokens/full-access
    type: Opaque
    data:
      # NOTE: the declared "full-access" token is missing as it was denied
      read-only-token-type: Bearer
      read-only-token-secret: JwBcd456.. # JWT token
      employee-client-id: 67b86a55-61e6-4862-aa14-70fe7be788f4
      employee-client-secret: 5585942c-ce79-44e4-aac2-8af565b51d3e

The ``zalando.org/problems`` annotation contains a list of "Problem JSON" objects as defined in `RFC 7807`_ as YAML.
At least the fields ``type``, ``title`` and ``instance`` should be set by the component processing the ``PlatformCredentialsSet`` resource:

``type``
    Machine-readable URI reference that identifies the problem type (e.g. https://example.org/invalid-grant)
``title``
    Short, human-readable summary of the problem type (e.g. "Invalid client grant")
``instance``
    Relative path indicating the problem location, this should reference the token or client (e.g. ``clients/my-client``)

See also the `Problem OpenAPI schema YAML`_.


.. _Kio: http://docs.stups.io/en/latest/components/kio.html
.. _YOUR TURN frontend: https://yourturn.stups.zalan.do/
.. _RFC 7807: https://tools.ietf.org/html/rfc7807
.. _Problem OpenAPI schema YAML: https://zalando.github.io/problem/schema.yaml
.. _supported token libraries: http://docs.stups.io/en/latest/appendix/oauth-integrations.html
