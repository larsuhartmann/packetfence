package pf::services::manager::api_frontend;

=head1 NAME

pf::services::manager::api_frontend - The service manager for the api_frontend  service

=cut

=head1 DESCRIPTION

pf::services::manager::api_frontend

=cut

use strict;
use warnings;
use Moo;
use pf::cluster;

extends 'pf::services::manager';

has '+name' => ( default => sub { 'api-frontend' } );

=head1 AUTHOR

Inverse inc. <info@inverse.ca>


=head1 COPYRIGHT

Copyright (C) 2005-2017 Inverse inc.

=head1 LICENSE

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation; either version 2
of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program; if not, write to the Free Software
Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301,
USA.

=cut

1;
