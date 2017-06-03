This is a support tool for those who already maintain their books with tally
and now interested in switching to ledger.

* For now, converts only the daybook from tally.
* For now, accepts daybook exported in text format.
* Works with Tally.ERP9.

How to export daybook in text format
------------------------------------

* Open tally
* Select the company whose daybook needs to be exported.
* Goto Display -> Daybook
* Select the period ``Alt-f2``
* Export ``Alt-e``

Subsequently Tally will show menu page with export options, in which
select the following options.

* ``Language:`` Restricted (ASCII only)
* ``Format:`` ASCII (comma delimited)
* ``Format:`` Detailed
* ``Show Voucher Numbers also:`` Yes
* ``Show Narrations also:`` Yes
* ``Show Billwise Details also:`` Yes
* ``Show Cost Centre Details also:`` Yes
* ``Show Inventory Details also:`` Yes
* ``Show Bank also:`` Yes

**Note1** We are yet to add support for multi-language support (option
``Language``)

Now build and execute as follows:

```bash
$ make install
$ tally2ledger -o <filename>.ldg -text <exporteddaybook>.txt <optional-rewrite>
```

There is also an undocumented feature called account-rewrite, that can rewrite
account names in transaction while generating ledger file.
