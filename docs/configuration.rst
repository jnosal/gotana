Configuration
=============

project
-------
Default: ``This parameter is mandatory``

::

    Name of the project used internally by the engine


outfilename
-----------
Default: ``Optional parameter``

::

    Path to file items will be saved to, see golangweekly.go example.



tcpaddress
----------
Default: ``Optional parameter``

::

    Host and Port combination that telnet console will bind to, e.g: localhost:7654


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


Example configuration
=====================

::

    project: test
    tcpaddress: localhost:7654
    outfilename: data.csv
    scrapers:
    - name: golang
      url: http://golangweekly.com
      requestlimit: 200
    - name: scrapinghub
      url: https://blog.scrapinghub.com
      requestlimit: 200