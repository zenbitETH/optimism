// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

import { ITournament } from "interfaces/dispute/ITournament.sol";
import { IDisputeGame } from "interfaces/dispute/IDisputeGame.sol";

/**
 * @title Tournament
 * @notice Implements a tournament-based dispute resolution mechanism
 * @dev Implements both ITournament and IDisputeGame interfaces
 */
contract Tournament is ITournament, IDisputeGame {
    /// @notice The root claim being disputed
    bytes32 public rootClaim;

    /// @notice The challenge period duration in seconds
    uint256 public challengePeriod;

    /// @notice The bond required to participate in the tournament
    uint256 public bondSize;

    /// @notice The creator of the tournament
    address public creator;

    /// @notice The timestamp when the tournament was created
    uint64 public override createdAt;

    /// @notice The current status of the tournament
    GameStatus private _status;

    /// @notice The L2 block number associated with this dispute
    uint256 private _l2BlockNumber;

    /// @notice Array of all nodes in the tournament
    Node[] private _nodes;

    /// @notice Array of all matches in the tournament
    Match[] private _matches;

    /// @notice Mapping of claim hashes to their node indices
    mapping(bytes32 => uint256) public claimIndices;

    /// @notice Mapping of addresses to their bond balances
    mapping(address => uint256) public bonds;

    /**
     * @notice Constructor for the Tournament contract
     * @param _rootClaim The root claim being disputed
     * @param _challengePeriod The challenge period duration in seconds
     * @param _bondSize The bond required to participate in the tournament
     * @param _creator The creator of the tournament
     * @param _inl2BlockNumber The L2 block number associated with this dispute
     */
    constructor(
        bytes32 _rootClaim,
        uint256 _challengePeriod,
        uint256 _bondSize,
        address _creator,
        uint256 _inl2BlockNumber
    ) {
        rootClaim = _rootClaim;
        challengePeriod = _challengePeriod;
        bondSize = _bondSize;
        creator = _creator;
        createdAt = uint64(block.timestamp);
        _status = GameStatus.IN_PROGRESS;
        _l2BlockNumber = _inl2BlockNumber;

        // Initialize with root node
        _nodes.push(Node({
            claim: _rootClaim,
            claimant: _creator,
            timestamp: uint64(block.timestamp),
            challenged: false
        }));

        claimIndices[_rootClaim] = 0;

        emit ClaimAdded(0, _rootClaim, _creator);
    }

    /**
     * @notice Join the tournament with a counter-claim
     * @param _claim The claim to submit
     * @param _parentClaimIndex The index of the parent claim being challenged
     * @return The index of the newly created node
     */
    function joinTournament(
        bytes32 _claim,
        uint256 _parentClaimIndex
    ) external payable override returns (uint256) {
        require(_status == GameStatus.IN_PROGRESS, "Tournament not in progress");
        require(_parentClaimIndex < _nodes.length, "Invalid parent claim index");
        require(!_nodes[_parentClaimIndex].challenged, "Claim already challenged");
        require(msg.value >= bondSize, "Insufficient bond");

        // Store bond
        bonds[msg.sender] += msg.value;

        // Create new node
        uint256 nodeIndex = _nodes.length;
        _nodes.push(Node({
            claim: _claim,
            claimant: msg.sender,
            timestamp: uint64(block.timestamp),
            challenged: false
        }));

        claimIndices[_claim] = nodeIndex;

        // Mark parent as challenged
        _nodes[_parentClaimIndex].challenged = true;

        // Create match
        uint256 matchIndex = _matches.length;
        _matches.push(Match({
            nodeIndex1: _parentClaimIndex,
            nodeIndex2: nodeIndex,
            deadline: uint64(block.timestamp + challengePeriod),
            winner: 0,
            evidence: bytes32(0)
        }));

        emit ClaimAdded(nodeIndex, _claim, msg.sender);
        emit MatchCreated(matchIndex, _parentClaimIndex, nodeIndex);

        return nodeIndex;
    }

    /**
     * @notice Submit evidence for a match
     * @param _matchIndex The index of the match
     * @param _evidence The evidence hash
     */
    function submitEvidence(uint256 _matchIndex, bytes32 _evidence) external override {
        require(_status == GameStatus.IN_PROGRESS, "Tournament not in progress");
        require(_matchIndex < _matches.length, "Invalid match index");

        Match storage xMatch = _matches[_matchIndex];
        require(xMatch.winner == 0, "Match already resolved");

        // Verify sender is a participant in the match
        Node storage node1 = _nodes[xMatch.nodeIndex1];
        Node storage node2 = _nodes[xMatch.nodeIndex2];
        require(
            msg.sender == node1.claimant || msg.sender == node2.claimant,
            "Not a participant in this match"
        );

        // Store evidence
        xMatch.evidence = _evidence;
    }

    /**
     * @notice Resolve a match in the tournament
     * @param _matchIndex The index of the match to resolve
     */
    function resolveMatch(uint256 _matchIndex) external override {
        require(_status == GameStatus.IN_PROGRESS, "Tournament not in progress");
        require(_matchIndex < _matches.length, "Invalid match index");

        Match storage xMatch = _matches[_matchIndex];
        require(xMatch.winner == 0, "Match already resolved");

        // Check if deadline has passed
        if (block.timestamp > xMatch.deadline) {
            // Default winner is the first node (defender)
            xMatch.winner = xMatch.nodeIndex1;
        } else {
            // In a real implementation, this would involve verification of evidence
            // For now, this is a placeholder that defaults to the defender
            xMatch.winner = xMatch.nodeIndex1;
        }

        emit MatchResolved(_matchIndex, xMatch.winner);

        // Check if tournament is complete
        checkTournamentCompletion();
    }

    /**
     * @notice Check if the tournament is complete
     */
    function checkTournamentCompletion() internal {
        bool allMatchesResolved = true;

        for (uint256 i = 0; i < _matches.length; i++) {
            if (_matches[i].winner == 0) {
                allMatchesResolved = false;
                break;
            }
        }

        if (allMatchesResolved && _matches.length > 0) {
            // Find the ultimate winner
            uint256 winnerIndex = 0;
            for (uint256 i = 0; i < _matches.length; i++) {
                if (_matches[i].nodeIndex1 == winnerIndex || _matches[i].nodeIndex2 == winnerIndex) {
                    winnerIndex = _matches[i].winner;
                }
            }

            // Determine game status based on winner
            if (winnerIndex == 0) {
                // Root claim wins (defender)
                _status = GameStatus.DEFENDER_WINS;
            } else {
                // Challenger wins
                _status = GameStatus.CHALLENGER_WINS;
            }

            // Emit tournament resolved event
            emit TournamentResolved(_status, _nodes[winnerIndex].claim);

            // Distribute bonds
            distributeBonds();
        }
    }

    /**
     * @notice Distribute bonds to winners
     */
    function distributeBonds() internal {
        // In a real implementation, this would distribute bonds based on the tournament outcome
        // For now, this is a placeholder

        // Return bond to the winner
        address winner = _nodes[0].claimant; // Default to defender
        if (_status == GameStatus.CHALLENGER_WINS) {
            // Find the ultimate winner
            uint256 winnerIndex = 0;
            for (uint256 i = 0; i < _matches.length; i++) {
                if (_matches[i].nodeIndex1 == winnerIndex || _matches[i].nodeIndex2 == winnerIndex) {
                    winnerIndex = _matches[i].winner;
                }
            }
            winner = _nodes[winnerIndex].claimant;
        }

        uint256 winnerBond = bonds[winner];
        if (winnerBond > 0) {
            bonds[winner] = 0;
            payable(winner).transfer(winnerBond);
        }
    }

    /**
     * @notice Get the result of the tournament
     * @return The status of the game and the winning claim
     */
    function result() external view override returns (GameStatus, bytes32) {
        if (_status == GameStatus.IN_PROGRESS) {
            return (_status, bytes32(0));
        }

        // Find the ultimate winner
        uint256 winnerIndex = 0;
        for (uint256 i = 0; i < _matches.length; i++) {
            if (_matches[i].nodeIndex1 == winnerIndex || _matches[i].nodeIndex2 == winnerIndex) {
                winnerIndex = _matches[i].winner;
            }
        }

        return (_status, _nodes[winnerIndex].claim);
    }

    /**
     * @notice Resolve the dispute game
     * @return The status of the game after resolution
     */
    function resolve() external override returns (GameStatus) {
        require(_status == GameStatus.IN_PROGRESS, "Game not in progress");

        // Check if challenge period has passed for all matches
        bool canResolve = true;
        for (uint256 i = 0; i < _matches.length; i++) {
            if (_matches[i].winner == 0 && block.timestamp <= _matches[i].deadline) {
                canResolve = false;
                break;
            }
        }

        require(canResolve, "Cannot resolve yet");

        // Resolve all unresolved matches
        for (uint256 i = 0; i < _matches.length; i++) {
            if (_matches[i].winner == 0) {
                _matches[i].winner = _matches[i].nodeIndex1; // Default to defender
                emit MatchResolved(i, _matches[i].nodeIndex1);
            }
        }

        // Determine game status
        _status = GameStatus.DEFENDER_WINS; // Default to defender wins

        // Emit tournament resolved event
        emit TournamentResolved(_status, _nodes[0].claim);

        // Distribute bonds
        distributeBonds();

        return _status;
    }

    /**
     * @notice Get the current status of the game
     * @return The current game status
     */
    function status() external view override returns (GameStatus) {
        return _status;
    }

    /**
     * @notice Get the L2 block number associated with this dispute
     * @return The L2 block number
     */
    function l2BlockNumber() external view override returns (uint256) {
        return _l2BlockNumber;
    }

    /**
     * @notice Get the number of nodes in the tournament
     * @return The number of nodes
     */
    function getNodeCount() external view override returns (uint256) {
        return _nodes.length;
    }

    /**
     * @notice Get a node by index
     * @param index The index of the node
     * @return The node
     */
    function getNode(uint256 index) external view override returns (Node memory) {
        require(index < _nodes.length, "Invalid node index");
        return _nodes[index];
    }

    /**
     * @notice Get the number of matches in the tournament
     * @return The number of matches
     */
    function getMatchCount() external view override returns (uint256) {
        return _matches.length;
    }

    /**
     * @notice Get a match by index
     * @param index The index of the match
     * @return The match
     */
    function getMatch(uint256 index) external view override returns (Match memory) {
        require(index < _matches.length, "Invalid match index");
        return _matches[index];
    }
}