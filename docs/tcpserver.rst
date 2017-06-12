==========
TCP Server
==========

Gotana offers telnet console for inspecting crawlers and controlling the engine.
The idea behind this service is to offer simple remote control over the scrapers.



Available commands
==================

+--------+---------------------------------------------------------------------------+
| Name   | Description                                                               |
+========+===========================================================================+
| HELP   | Displays list of available commands                                       |
+--------+---------------------------------------------------------------------------+
| LIST   | Displays lists of available scrapers                                      |
+--------+---------------------------------------------------------------------------+
| STATS  | Displays statisting regarding scrapers: fetched item, requests, etc...    |
+--------+---------------------------------------------------------------------------+
| RESUME | Starts engine after pause (awaits scrapers cleanup)                       |
+--------+---------------------------------------------------------------------------+


Usage
=====

::

    telnet localhost 6023
    HELP
    -------------------------------------
    Available commands: LIST, STATS, HELP, STOP
    STATS
    -------------------------------------
    Total scrapers: 1. Total requests: 45
    -------------------------------------
    ------------------------------------------------------------------------------------------
    <Scraper: golangweekly.com>. Crawled: 45, successful: 44, failed: 1. Scraped: 44, saved: 9
    ------------------------------------------------------------------------------------------
    --------------------------------------------------------
    Currently fetching: http://golangweekly.com/rss/14p9ef33
    --------------------------------------------------------
    LIST
    ------------------------
    Running scrapers: golang
    STOP
    --------------------
    Stopping scrapers...
