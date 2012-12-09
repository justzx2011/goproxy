#!/usr/bin/python
# -*- coding: utf-8 -*-
'''
@date: 2012-11-08
@author: shell.xu
'''
import os, sys

with open("/dev/random", 'rb') as fi: data = fi.read(32)
with open(sys.argv[1], 'wb') as fo: fo.write(data)
