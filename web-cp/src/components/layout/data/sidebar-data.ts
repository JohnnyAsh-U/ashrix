import {
  LayoutDashboard,
  LayoutGrid,
  ChevronsLeftRightEllipsis,
  Hexagon,
  Columns4,
  Users,
  ShieldPlus,
  ScrollText,
  Settings,
} from 'lucide-react'
import { type SidebarData } from '../types'
import { useAuthStore } from '@/stores/auth.store'

export const sidebarData = (): SidebarData => {
  return {
    user: {
      name: useAuthStore().user?.username || '',
      email: useAuthStore().user?.email || '',
      avatar: '/avatars/shadcn.jpg',
    },
    navGroups: [
      {
        title: 'GENERAL',
        items: [
          {
            title: 'Tableau de bord',
            url: '/dashboard',
            icon: LayoutDashboard,
          },
        ],
      },
      {
        title: 'INFRASTRUCTURE',
        items: [
          {
            title: 'Applications',
            url: '/applications',
            icon: LayoutGrid,
          },
          {
            title: 'Connecteurs',
            url: '/connectors',
            icon: ChevronsLeftRightEllipsis ,
          },
          {
            title: 'Passerelles',
            url: '/gateways',
            icon: Hexagon,
          },
        ],
      },
      {
        title: 'CONTROLE D\'ACCES',
        items: [
          {
            title: 'Politiques',
            url: '/policies',
            icon: Columns4,
          },
          {
            title: 'Utilisateurs',
            url: '/users',
            icon: Users,
          },
          {
            title: 'Sessions',
            url: '/sessions',
            icon: ShieldPlus,
          },
          {
            title: 'Logs d\'accès',
            url: '/access-logs',
            icon: ScrollText,
          },
        ],
      },
      {
        title: "AUTRES",
        items: [
          {
            title: "Parametres",
            url: '/settings',
            icon: Settings,
          }
        ],
      }
    ],
  }
}

