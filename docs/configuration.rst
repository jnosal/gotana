Configuration
=============

project
-------
Default: ``This parameter is mandatory``

::

    Name of the project used internally by the engine


tcpaddress
----------
Default: ``Optional parameter``

::

    Host and Port combination that telnet console will bind to, e.g: localhost:7654


httpaddress
-----------
Default: ``Optional parameter``

::

    Host and Port combination that HTTP API server will bind to, e.g: localhost:5555


redisaddress
------------
Default: ``Optional parameter``

::

    Host and Port combination of redis server, which is required for http api frontend as well as storage.


scrapers
--------
Default: ``This parameter is mandatory``

::

    List of scrapers that will be executed by the engine


Scraper Configuration
=====================

name
----
Default: ``This parameter is mandatory``

::

    Internal name of the scraper


url
----

Default: ``This parameter is mandatory``

::

    Base url which will be used to start crawling


requestlimit
------------
Default: ``1 millisecond``

::

    Number of millisecond to wait between requests



patterns
--------
Default: ``Optional parameter``

::

    List of patterns to validate url that's currently being scraped against. See patterns configuration.


extractor
---------
Default: ``Optional parameter``

::

    Short name of extractor struct which implements Extractable interface, by defualt LinkExtractor (link) is used.


Patterns Configuration
======================

type
----
Default: ``This parameter is mandatory``

::

    Either contains or regexp. First one uses string matching, the latter relies on regular expression.


pattern
-------

Default: ``This parameter is mandatory``

::

    Value that's used as string to match against or regexp expression depending on the type of pattern.

Example configuration
=====================

::

    project: test
    tcpaddress: localhost:7654
    redisaddress: localhost:6379
    httpaddress: localhost:5555
    scrapers:
    - name: golang
      url: http://golangweekly.com
      requestlimit: 200
      patterns:
      - type: contains
        pattern: /issues
    - name: scrapinghub
      url: https://blog.scrapinghub.com
      requestlimit: 200
