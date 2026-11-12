import { createFileRoute } from '@tanstack/react-router'

import { BoardPage } from '../../features/shell/pages'

export const Route = createFileRoute('/_agent/board')({ component: BoardPage })
